#ifndef GO1SIM_ODOMETRY_FRAME_HPP_
#define GO1SIM_ODOMETRY_FRAME_HPP_
#include "observation_frame.hpp"

namespace simulation_state {
// ODO1: same 160-byte prefix as OBS1, 85 doubles, CRC32C. 844 bytes total.
// Values: position xyz, quaternion xyzw, linear xyz, angular xyz,
// pose covariance[36], twist covariance[36]. No pointers or native C++ structs on wire.
struct Odometry {
  std::uint64_t sim_ns{0}, steady_ns{0}, sequence{0};
  std::string parent, child;
  std::array<double, 85> values{};
};
constexpr std::size_t odometry_bytes = 844;
inline bool encodeOdometry(const Odometry & o, std::array<std::uint8_t, odometry_bytes> & data) {
  using namespace go1sim_relay;
  if (o.parent.size() >= 64 || o.child.size() >= 64) {return false;}
  data.fill(0);
  putU32(data.data(), 0x314f444fU); putU32(data.data()+4, 3);
  putU64(data.data()+8, o.sim_ns); putU64(data.data()+16, o.steady_ns);
  putU64(data.data()+24, o.sequence);
  std::memcpy(data.data()+32, o.parent.data(), o.parent.size());
  std::memcpy(data.data()+96, o.child.data(), o.child.size());
  for (std::size_t i=0; i<o.values.size(); ++i) {
    if (!std::isfinite(o.values[i])) {return false;}
    std::uint64_t bits; std::memcpy(&bits, &o.values[i], 8);
    putU64(data.data()+160+8*i, bits);
  }
  putU32(data.data()+840, crc32c(data.data(), 840)); return true;
}
inline bool decodeOdometry(const std::uint8_t * data, std::size_t size, Odometry & o) {
  using namespace go1sim_relay;
  if (size != odometry_bytes || getU32(data) != 0x314f444fU || getU32(data+4) != 3 ||
      getU32(data+840) != crc32c(data, 840) || !std::memchr(data+32,0,64) ||
      !std::memchr(data+96,0,64)) {return false;}
  o.sim_ns=getU64(data+8); o.steady_ns=getU64(data+16); o.sequence=getU64(data+24);
  o.parent=reinterpret_cast<const char *>(data+32); o.child=reinterpret_cast<const char *>(data+96);
  for (std::size_t i=0; i<o.values.size(); ++i) {
    const auto bits=getU64(data+160+8*i); std::memcpy(&o.values[i], &bits, 8);
    if (!std::isfinite(o.values[i])) {return false;}
  }
  return true;
}
}
#endif
