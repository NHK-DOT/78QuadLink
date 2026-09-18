#ifndef GO1SIM_SHARED_STATE_BOARD_HPP_
#define GO1SIM_SHARED_STATE_BOARD_HPP_

#include <cstdint>
#include <cstddef>
#include <cstring>
#include <fcntl.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <unistd.h>

namespace simulation_state
{
// Linux GCC/Clang ABI, native 64-bit lock-free atomics. Independent of robot schema.
// One writer per partition, multiple readers. No global multi-partition transaction.
class Board
{
public:
  static constexpr std::size_t slots = 8;
  static constexpr std::size_t capacity = 1024;
  static constexpr std::size_t stride = 1088;
  static constexpr std::size_t bytes = 64 + slots * stride;
  static constexpr std::uint64_t magic = 0x31564452414f4253ULL;
  static_assert(__atomic_always_lock_free(8, nullptr), "64-bit atomics must be lock-free");

  Board() = default;
  Board(const Board &) = delete;
  Board & operator=(const Board &) = delete;
  ~Board() {close();}

  bool open(const char * path, std::size_t writer_slot = slots)
  {
    if (memory_) {return writer_ == writer_slot;}
    if (!path || writer_slot > slots) {return false;}
    fd_ = ::open(path, (writer_slot == slots ? O_RDONLY : O_RDWR) | O_CLOEXEC | O_NOFOLLOW);
    if (fd_ < 0) {return false;}
    struct stat status{};
    if (::fstat(fd_, &status) || !S_ISREG(status.st_mode) || status.st_size != bytes) {
      close(); return false;
    }
    // Startup-only writer ownership lock, released by close/process exit.
    struct flock lock{};
    lock.l_type = F_WRLCK;
    lock.l_whence = SEEK_SET;
    lock.l_start = 64 + writer_slot * stride;
    lock.l_len = stride;
    if (writer_slot < slots && ::fcntl(fd_, F_OFD_SETLK, &lock) < 0) {
      close(); return false;
    }
    void * mapping = ::mmap(nullptr, bytes,
      writer_slot == slots ? PROT_READ : PROT_READ | PROT_WRITE, MAP_SHARED, fd_, 0);
    if (mapping == MAP_FAILED) {close(); return false;}
    memory_ = static_cast<std::uint8_t *>(mapping);
    std::uint64_t version = 0;
    std::memcpy(&version, memory_, sizeof(version));
    if (version != magic) {close(); return false;}
    writer_ = writer_slot;
    return true;
  }

  void close()
  {
    if (memory_) {::munmap(memory_, bytes); memory_ = nullptr;}
    if (fd_ >= 0) {::close(fd_); fd_ = -1;}
  }

  bool publish(const std::uint8_t * data, std::size_t size)
  {
    if (!memory_ || writer_ >= slots || !size || size > capacity) {return false;}
    auto * words = partition(writer_);
    // All shared payload accesses are atomic: ordinary-field seqlock has data races.
    // seq_cst deliberately favors a simple correctness baseline over peak throughput.
    const auto previous = __atomic_load_n(words, __ATOMIC_SEQ_CST);
    const auto next = (previous | 1ULL) + 1ULL;
    __atomic_store_n(words, next - 1, __ATOMIC_SEQ_CST);
    __atomic_store_n(words + 1, static_cast<std::uint64_t>(size), __ATOMIC_SEQ_CST);
    for (std::size_t offset = 0; offset < size; offset += 8) {
      std::uint64_t value = 0;
      const auto count = size - offset < 8 ? size - offset : 8;
      std::memcpy(&value, data + offset, count);
      __atomic_store_n(words + 2 + offset / 8, value, __ATOMIC_SEQ_CST);
    }
    __atomic_store_n(words, next, __ATOMIC_SEQ_CST);
    return true;
  }

  // Three bounded attempts; leave caller's previous snapshot intact on contention.
  bool snapshot(std::size_t slot, std::uint8_t * output, std::size_t available,
    std::size_t & size, std::uint64_t & last_revision) const
  {
    if (!memory_ || slot >= slots) {return false;}
    auto * words = partition(slot);
    std::uint8_t scratch[capacity];
    for (int attempt = 0; attempt < 3; ++attempt) {
      const auto before = __atomic_load_n(words, __ATOMIC_SEQ_CST);
      if (!before || before == last_revision) {return false;}
      if (before & 1U) {continue;}
      const auto length = __atomic_load_n(words + 1, __ATOMIC_SEQ_CST);
      if (!length || length > capacity || length > available) {return false;}
      for (std::size_t offset = 0; offset < length; offset += 8) {
        const auto value = __atomic_load_n(words + 2 + offset / 8, __ATOMIC_SEQ_CST);
        const auto count = length - offset < 8 ? length - offset : 8;
        std::memcpy(scratch + offset, &value, count);
      }
      if (__atomic_load_n(words, __ATOMIC_SEQ_CST) == before) {
        std::memcpy(output, scratch, length);
        size = length;
        last_revision = before;
        return true;
      }
    }
    return false;
  }

private:
  std::uint64_t * partition(std::size_t slot) const
  {return reinterpret_cast<std::uint64_t *>(memory_ + 64 + slot * stride);}
  int fd_{-1};
  std::uint8_t * memory_{nullptr};
  std::size_t writer_{0};
};
}  // namespace simulation_state
#endif
