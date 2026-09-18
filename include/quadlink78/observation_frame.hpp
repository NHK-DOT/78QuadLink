#ifndef GO1SIM_OBSERVATION_FRAME_HPP_
#define GO1SIM_OBSERVATION_FRAME_HPP_
#include <array>
#include <cmath>
#include <chrono>
#include <string>
#include "relay_protocol.hpp"

namespace simulation_state {
// Version 1 little-endian observation envelope. Type 1: IMU, type 2: one TF edge.
// IMU values: quaternion wxyz, angular velocity xyz, linear acceleration xyz.
// TF values: translation xyz, quaternion xyzw, then three reserved zeros.
struct Observation {
  std::uint32_t type{0};
  std::uint64_t sim_ns{0}, steady_ns{0}, sequence{0};
  std::string parent, child;
  std::array<double, 10> values{};
};
constexpr std::size_t observation_bytes = 244;
inline std::uint64_t steadyNow() {
  return static_cast<std::uint64_t>(std::chrono::duration_cast<std::chrono::nanoseconds>(
    std::chrono::steady_clock::now().time_since_epoch()).count());
}
inline bool encodeObservation(const Observation & o,
  std::array<std::uint8_t, observation_bytes> & data) {
  using namespace go1sim_relay;
  if (o.parent.size() >= 64 || o.child.size() >= 64 || (o.type != 1 && o.type != 2)) {
    return false;
  }
  data.fill(0);
  putU32(data.data(), 0x3142534fU);
  putU32(data.data() + 4, o.type);
  putU64(data.data() + 8, o.sim_ns);
  putU64(data.data() + 16, o.steady_ns);
  putU64(data.data() + 24, o.sequence);
  std::memcpy(data.data() + 32, o.parent.data(), o.parent.size());
  std::memcpy(data.data() + 96, o.child.data(), o.child.size());
  for (std::size_t i = 0; i < 10; ++i) {
    if (!std::isfinite(o.values[i])) {return false;}
    std::uint64_t bits;
    std::memcpy(&bits, &o.values[i], 8);
    putU64(data.data() + 160 + 8*i, bits);
  }
  putU32(data.data() + 240, crc32c(data.data(), 240));
  return true;
}
inline bool decodeObservation(const std::uint8_t * data, std::size_t size, Observation & o) {
  using namespace go1sim_relay;
  if (size != observation_bytes || getU32(data) != 0x3142534fU ||
      getU32(data + 240) != crc32c(data, 240)) {return false;}
  o.type = getU32(data + 4);
  if (o.type != 1 && o.type != 2) {return false;}
  o.sim_ns = getU64(data + 8); o.steady_ns = getU64(data + 16);
  o.sequence = getU64(data + 24);
  if (!std::memchr(data + 32, 0, 64) || !std::memchr(data + 96, 0, 64)) {return false;}
  o.parent = reinterpret_cast<const char *>(data + 32);
  o.child = reinterpret_cast<const char *>(data + 96);
  for (std::size_t i = 0; i < 10; ++i) {
    const auto bits = getU64(data + 160 + 8*i);
    std::memcpy(&o.values[i], &bits, 8);
    if (!std::isfinite(o.values[i])) {return false;}
  }
  return true;
}
}
#endif
