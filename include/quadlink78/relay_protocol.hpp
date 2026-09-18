#ifndef ROS2_UNITREE_LEGGED_MSGS__RELAY_PROTOCOL_HPP_
#define ROS2_UNITREE_LEGGED_MSGS__RELAY_PROTOCOL_HPP_

#include <array>
#include <cstddef>
#include <cstdint>
#include <cstring>

namespace go1sim_relay
{

constexpr std::uint32_t kMagic = 0x4731534DU;
constexpr std::uint8_t kVersion = 2;
constexpr std::size_t kJointCount = 12;
constexpr std::size_t kWireHeaderSize = 36;
constexpr std::size_t kWireCrcSize = 4;
constexpr std::size_t kCommandPayloadSize = kJointCount * 24;
constexpr std::size_t kStatePayloadSize = kJointCount * 20;
constexpr std::size_t kControlPacketSize = kWireHeaderSize + kWireCrcSize;
constexpr std::size_t kCommandPacketSize = kWireHeaderSize + kCommandPayloadSize + kWireCrcSize;
constexpr std::size_t kStatePacketSize = kWireHeaderSize + kStatePayloadSize + kWireCrcSize;
constexpr std::size_t kMaxPacketSize = kCommandPacketSize;

enum class MessageType : std::uint8_t
{
  Register = 1,
  Unregister = 2,
  Command = 3,
  State = 4,
};

enum class Role : std::uint8_t
{
  Guide = 1,
  Controller = 2,
};

enum class DecodeError : std::uint8_t
{
  None = 0,
  Size,
  Magic,
  Version,
  Flags,
  Type,
  Role,
  PayloadSize,
  Crc,
};

struct Header
{
  MessageType type{MessageType::Register};
  Role role{Role::Guide};
  std::uint64_t sequence{0};
  std::uint64_t timestamp_ns{0};
  std::uint64_t session_id{0};
};

struct MotorCommand
{
  std::uint32_t mode{0};
  float q{0.0F};
  float dq{0.0F};
  float tau{0.0F};
  float kp{0.0F};
  float kd{0.0F};
};

struct MotorState
{
  std::uint32_t mode{0};
  float q{0.0F};
  float dq{0.0F};
  float ddq{0.0F};
  float tau_est{0.0F};
};

struct CommandFrame
{
  Header header{};
  std::array<MotorCommand, kJointCount> motors{};
};

struct StateFrame
{
  Header header{};
  std::array<MotorState, kJointCount> motors{};
};

inline void putU32(std::uint8_t * output, std::uint32_t value)
{
  output[0] = static_cast<std::uint8_t>(value);
  output[1] = static_cast<std::uint8_t>(value >> 8U);
  output[2] = static_cast<std::uint8_t>(value >> 16U);
  output[3] = static_cast<std::uint8_t>(value >> 24U);
}

inline void putU64(std::uint8_t * output, std::uint64_t value)
{
  for (std::size_t i = 0; i < 8; ++i) {
    output[i] = static_cast<std::uint8_t>(value >> (i * 8U));
  }
}

inline std::uint32_t getU32(const std::uint8_t * input)
{
  return static_cast<std::uint32_t>(input[0]) |
         (static_cast<std::uint32_t>(input[1]) << 8U) |
         (static_cast<std::uint32_t>(input[2]) << 16U) |
         (static_cast<std::uint32_t>(input[3]) << 24U);
}

inline std::uint64_t getU64(const std::uint8_t * input)
{
  std::uint64_t value = 0;
  for (std::size_t i = 0; i < 8; ++i) {
    value |= static_cast<std::uint64_t>(input[i]) << (i * 8U);
  }
  return value;
}

inline void putFloat(std::uint8_t * output, float value)
{
  std::uint32_t bits = 0;
  std::memcpy(&bits, &value, sizeof(bits));
  putU32(output, bits);
}

inline float getFloat(const std::uint8_t * input)
{
  const std::uint32_t bits = getU32(input);
  float value = 0.0F;
  std::memcpy(&value, &bits, sizeof(value));
  return value;
}

inline const std::array<std::uint32_t, 256> & crc32cTable()
{
  static const std::array<std::uint32_t, 256> table = []() {
      std::array<std::uint32_t, 256> result{};
      constexpr std::uint32_t polynomial = 0x82F63B78U;
      for (std::size_t i = 0; i < result.size(); ++i) {
        std::uint32_t crc = static_cast<std::uint32_t>(i);
        for (std::size_t bit = 0; bit < 8; ++bit) {
          crc = (crc >> 1U) ^ ((crc & 1U) != 0U ? polynomial : 0U);
        }
        result[i] = crc;
      }
      return result;
    }();
  return table;
}

inline std::uint32_t crc32c(const std::uint8_t * data, std::size_t size)
{
  std::uint32_t crc = 0xFFFFFFFFU;
  const auto & table = crc32cTable();
  for (std::size_t i = 0; i < size; ++i) {
    crc = table[(crc ^ data[i]) & 0xFFU] ^ (crc >> 8U);
  }
  return ~crc;
}

inline bool validRole(Role role)
{
  return role == Role::Guide || role == Role::Controller;
}

inline bool validType(MessageType type)
{
  return type == MessageType::Register || type == MessageType::Unregister ||
         type == MessageType::Command || type == MessageType::State;
}

inline void encodeHeader(
  const Header & header, std::uint32_t payload_size, std::uint8_t * output)
{
  putU32(output, kMagic);
  output[4] = kVersion;
  output[5] = static_cast<std::uint8_t>(header.type);
  output[6] = static_cast<std::uint8_t>(header.role);
  output[7] = 0;
  putU32(output + 8, payload_size);
  putU64(output + 12, header.sequence);
  putU64(output + 20, header.timestamp_ns);
  putU64(output + 28, header.session_id);
}

inline bool decodeHeader(
  const std::uint8_t * input, std::size_t size, Header & header,
  std::uint32_t & payload_size, DecodeError & error)
{
  if (size < kControlPacketSize) {
    error = DecodeError::Size;
    return false;
  }
  if (getU32(input) != kMagic) {
    error = DecodeError::Magic;
    return false;
  }
  if (input[4] != kVersion) {
    error = DecodeError::Version;
    return false;
  }
  if (input[7] != 0U) {
    error = DecodeError::Flags;
    return false;
  }
  header.type = static_cast<MessageType>(input[5]);
  header.role = static_cast<Role>(input[6]);
  if (!validType(header.type)) {
    error = DecodeError::Type;
    return false;
  }
  if (!validRole(header.role)) {
    error = DecodeError::Role;
    return false;
  }
  payload_size = getU32(input + 8);
  if (size != kWireHeaderSize + static_cast<std::size_t>(payload_size) + kWireCrcSize) {
    error = DecodeError::PayloadSize;
    return false;
  }
  const auto expected_crc = getU32(input + size - kWireCrcSize);
  if (crc32c(input, size - kWireCrcSize) != expected_crc) {
    error = DecodeError::Crc;
    return false;
  }
  header.sequence = getU64(input + 12);
  header.timestamp_ns = getU64(input + 20);
  header.session_id = getU64(input + 28);
  error = DecodeError::None;
  return true;
}

inline bool encodeControlPacket(
  const Header & header, std::uint8_t * output, std::size_t capacity, std::size_t & written)
{
  if (capacity < kControlPacketSize ||
    (header.type != MessageType::Register && header.type != MessageType::Unregister))
  {
    return false;
  }
  encodeHeader(header, 0, output);
  putU32(output + kWireHeaderSize, crc32c(output, kWireHeaderSize));
  written = kControlPacketSize;
  return true;
}

inline bool encodeFrame(
  const CommandFrame & frame, std::uint8_t * output, std::size_t capacity, std::size_t & written)
{
  if (capacity < kCommandPacketSize) {
    return false;
  }
  encodeHeader(frame.header, static_cast<std::uint32_t>(kCommandPayloadSize), output);
  std::size_t offset = kWireHeaderSize;
  for (const auto & motor : frame.motors) {
    putU32(output + offset, motor.mode);
    putFloat(output + offset + 4, motor.q);
    putFloat(output + offset + 8, motor.dq);
    putFloat(output + offset + 12, motor.tau);
    putFloat(output + offset + 16, motor.kp);
    putFloat(output + offset + 20, motor.kd);
    offset += 24;
  }
  putU32(output + offset, crc32c(output, offset));
  written = kCommandPacketSize;
  return true;
}

inline bool encodeFrame(
  const StateFrame & frame, std::uint8_t * output, std::size_t capacity, std::size_t & written)
{
  if (capacity < kStatePacketSize) {
    return false;
  }
  encodeHeader(frame.header, static_cast<std::uint32_t>(kStatePayloadSize), output);
  std::size_t offset = kWireHeaderSize;
  for (const auto & motor : frame.motors) {
    putU32(output + offset, motor.mode);
    putFloat(output + offset + 4, motor.q);
    putFloat(output + offset + 8, motor.dq);
    putFloat(output + offset + 12, motor.ddq);
    putFloat(output + offset + 16, motor.tau_est);
    offset += 20;
  }
  putU32(output + offset, crc32c(output, offset));
  written = kStatePacketSize;
  return true;
}

inline bool decodeFrame(
  const std::uint8_t * input, std::size_t size, CommandFrame & frame, DecodeError & error)
{
  std::uint32_t payload_size = 0;
  if (!decodeHeader(input, size, frame.header, payload_size, error)) {
    return false;
  }
  if (frame.header.type != MessageType::Command || frame.header.role != Role::Guide) {
    error = DecodeError::Type;
    return false;
  }
  if (payload_size != kCommandPayloadSize || size != kCommandPacketSize) {
    error = DecodeError::PayloadSize;
    return false;
  }
  std::size_t offset = kWireHeaderSize;
  for (auto & motor : frame.motors) {
    motor.mode = getU32(input + offset);
    motor.q = getFloat(input + offset + 4);
    motor.dq = getFloat(input + offset + 8);
    motor.tau = getFloat(input + offset + 12);
    motor.kp = getFloat(input + offset + 16);
    motor.kd = getFloat(input + offset + 20);
    offset += 24;
  }
  return true;
}

inline bool decodeFrame(
  const std::uint8_t * input, std::size_t size, StateFrame & frame, DecodeError & error)
{
  std::uint32_t payload_size = 0;
  if (!decodeHeader(input, size, frame.header, payload_size, error)) {
    return false;
  }
  if (frame.header.type != MessageType::State || frame.header.role != Role::Controller) {
    error = DecodeError::Type;
    return false;
  }
  if (payload_size != kStatePayloadSize || size != kStatePacketSize) {
    error = DecodeError::PayloadSize;
    return false;
  }
  std::size_t offset = kWireHeaderSize;
  for (auto & motor : frame.motors) {
    motor.mode = getU32(input + offset);
    motor.q = getFloat(input + offset + 4);
    motor.dq = getFloat(input + offset + 8);
    motor.ddq = getFloat(input + offset + 12);
    motor.tau_est = getFloat(input + offset + 16);
    offset += 20;
  }
  return true;
}

}  // namespace go1sim_relay

#endif  // ROS2_UNITREE_LEGGED_MSGS__RELAY_PROTOCOL_HPP_
