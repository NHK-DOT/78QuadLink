# 四足仿真与真机通信：已有方案、项目价值和原始基线

查阅日期：2026-09-19。证据主要来自厂商/框架官方开源代码与文档，保存于
`artifacts/research/v043/`；在线 main/master 会变化，不能等同于本机安装版本。
没有在本轮安装/运行 Isaac Lab、其他机器狗模型或操作真实机器人，以下不含跨平台实测排名。

## 判断

“减少本项目十二路重复调度”有已测量的收益，不是假需求；“重新发明共享内存、
整机聚合帧或通用通信中间件”没有新颖性，这些是成熟方法。
应该把本项目定位为可复现实验工具和机器人/仿真器适配层，而非自创总线标准。
若只是让单只狗实时运行，已经达到 RTF≈1 时，继续降低 CPU 是节省资源，不能
包装成走得更稳、控制延迟更小或强化学习训练更快。

## 其他仿真路线怎么做

| 公开项目 | 可核查的做法 | 对本项目的启示 |
|---|---|---|
| Unitree MuJoCo | SDK2 的 LowCmd/LowState，按整机电机数组循环处理；bridge 直接读取 MuJoCo sensor 数组、写 ctrl；保留 DDS 真机接口；可分别配置仿真与 viewer 步长 | 聚合早已是常规做法；兼容真机 SDK 可能比继续删掉标准接口更有价值 |
| ETH legged_gym / Isaac Gym（ANYmal 等） | 大量并行环境；GPU 物理缓冲直接到 PyTorch tensor；headless 训练 | 训练吞吐主要靠批处理、GPU 数据驻留和少渲染，不靠逐个关节 ROS topic 优化 |
| 云深处 rl_training（Lite3、M20 等） | 基于 Isaac Lab，支持 headless、num_envs、多 GPU/多节点训练 | 优化目标是环境步吞吐和训练，不等同于单只 Gazebo 实时仿真 |
| MuJoCo MJX | 把 model/data 放到设备，JAX jit/vmap 批量 step；另有 Warp GPU 实现 | 多环境吞吐路线；不是在同一个单狗 ROS 场景里换语言就必然更快 |

Unitree MuJoCo 的 C++ bridge 代码明确创建 `rt/lowcmd` 订阅与 LowState 发布，
按 `num_motor_` 循环计算 tau + kp*(q_des-q) + kd*(dq_des-dq)。这是读取同进程
MuJoCo 内存、再与外部控制器通过 DDS 通信；不要把普通指针访问说成跨进程共享内存。
其 README 推荐 C++ 版本，Python 示例分别列出 SIMULATE_DT 与 VIEWER_DT；这说明
计算/显示频率是需要单独配置的边界，不证明任意降低频率后控制质量不变。

Isaac Gym 论文里的训练加速量级来自其特定 GPU 训练任务，不能拿来作为本机 Go1
Gazebo 的预计提升。不同引擎的接触模型、传感器、时间步与任务也须分别验证。

## 真机是不是已经在用

需要分清三层：同一计算机上的进程间通信、上位机到机器人主控的网络通信、主控到驱动器的硬件通信。

| 公开证据 | 能确认 | 不能据此确认 |
|---|---|---|
| 宇树 unitree_legged_sdk | UDP 控制示例；LowCmd/LowState 为固定结构，包含 motor 数组与 CRC，头文件预留 20 个 motor 槽 | Go1 整机内部每段线路的物理协议；私有进程是否使用共享内存池 |
| 宇树 unitree_sdk2 Go2 示例 | 整个 LowCmd 的 motor_cmd 数组、CRC、DDS channel；不是十二个独立 ROS command topic | 所有宇树机型都用同一个低层协议，或该接口自动启用了同机零拷贝 |
| 云深处 Lite3_MotionSDK | 官方明确写 UDP，整体 RobotCmd/四条腿关节结构，包含位置、速度、kp、kd、力矩等参数；底层再分发给十二关节 | 所有云深处机型、所有内部总线的实现都相同 |
| 云深处 Deep_Motor_SDK | 公开 CAN 通信、单/多关节例程、can0 1 Mbit/s 设置与 CAN 指令/DLC 定义 | 每一款完整机器人内部都采用这套 SDK 和相同总线拓扑 |
| 宇树 unitree_actuator_sdk | 独立电机 API 有 serial.sendRecv；转子/输出轴参数需要齿比转换 | SDK 用户收到的 Ethernet/DDS 整机帧就是电机物理总线帧 |

所以“聚合二进制命令帧/真实 CAN 电机接口是否存在”的答案是存在；
“宇树、云深处所有真机内部是否用我们的这种共享板/内存池”没有足够公开证据，不能猜。
共享内存也不能直接跨到另一台计算机或电机 MCU 的私有 RAM；这些边界仍需网络/物理总线。

我们的 328 B 命令帧和 280 B 状态帧是**自定义主机 IPC 帧**，包含十二电机记录，
不是 CAN 帧，也没有实现 CAN 的仲裁、电气层或错误恢复。经典 CAN 数据段最多 8 B，
CAN FD 最多 64 B；“有头、字段、CRC”只是许多二进制协议共有的结构。
当前共享板更准确叫“固定分区最新值 mailbox”：不是有历史的队列，也不是通用
可分配/回收大块数据的共享内存池，仍有字段写入/快照读取，不能泛称端到端零拷贝。

## 共享内存池已经有哪些轮子

- **Fast DDS Data-sharing**：官方说明预分配共享内存 sample pool，读写双方共享
  writer history，减少 writer-reader 间拷贝；同时明确应用到 DDS reader/writer 的拷贝
  不会因此自动消失。普通 SHM transport 与 Data-sharing 是不同机制。
  文档列出 bounded、非 keyed、内存分配策略等约束，不能假设 Humble 任意消息自动适用。
- **iceoryx2**：公开的共享内存零拷贝、lock-free IPC 工具。可作为独立数据面的候选，
  但不等于在当前 ROS 2 Humble 上替换一个环境变量就得到完整集成。
- 当前环境使用的 RMW、消息 bounded 属性、loaned-message 支持和实际拷贝链路应先验证。
  “代码安装了 CycloneDDS”不证明共享内存已打开；“系统支持 SHM”也不证明热路径用了它。

下一步应先让成熟 middleware/同进程组合方案成为可选后端，做同场景对照。
只有定制共享板在维护成本、兼容性、CPU/尾延迟上有清晰优势时，才值得长期保留第二套协议。

## 最初十二路到底提升了多少

旧文档 [v0.2 报告](https://github.com/NHK-DOT/go1sim/blob/v0.5/docs/unitree_rt_controller_v0.2_report.md) 的同批十分钟测试（每组 600 样本）：

| 路径 | gzserver | junior_ctrl | Go relay | 以上进程合计 | 相比十二路降低 |
|---|---:|---:|---:|---:|---:|
| 十二路基线（87089be） | 70.42% | 23.39% | 0 | 93.81% | — |
| 聚合 DDS（b4f2696） | 40.20% | 11.54% | 0 | 51.74% | 44.8% |
| Go relay（4e157a2） | 34.86% | 9.42% | 3.84% | 48.12% | 48.7% |

这些数字是进程 CPU，不是端到端延迟或真机总功耗。十二路基线仍是 v0.1 的
UnitreeRealtimeController；更早的原 UnitreeLeggedController 在 v0.1 报告里只有
短时 ps 快照（gzserver 54.6%、junior_ctrl 22.3%），不应与后来的十分钟均值硬算增益。

今天保留的十二路模式同样是 v0.1 controller，附加与 v0.4.3 相同的固定站立、全进程
统计对照，见 [本轮性能报告](v043_performance.md)。本次夹心对照中，十二路 CPU 为 127.11% 与 99.00%，中间的当前共享路径为 49.33%，
对应降低 61.2% 与 50.2%（以两次基线均值比较为 56.4%），三组报告 RTF 均为 1.0。
由于基线自身波动明显，只能将其概括为本机该场景约减半。不会将旧“两三个进程合计”和
新“包含 TF/RSP 等全体本次进程合计”直接相减，也不会把不同轮次的百分比连乘。

## 合理的下一步

1. 将工程价值定义为“适配现有机器人/仿真器 + 可开关优化 + 可复核的性能/功能对照”。
2. 第二种狗优先选有官方模型和运动 SDK 的 Lite3，在配置层明确关节顺序、单位、
   转子/输出轴、控制周期；本轮仅调研，尚未把它宣称为已支持的 adapter。
3. 单狗 CPU/ROS 兼容路线：聚合、最新值语义、已有共享内存后端、少重复消费者，
   每一步单独测；odom 实验不省资源便默认关闭。
4. 大规模训练路线：优先验证 Isaac Lab/MJX 等成熟环境，衡量环境步/秒、每步成本与
   sim-to-real 表现，而不是继续针对 ROS topic 做小幅优化。
5. Go 负责跨后端工具、录制分析、批量基准和模型元数据适配；不用语言替换作为收益证明。

## 一手资料

1. [Unitree MuJoCo README](https://github.com/unitreerobotics/unitree_mujoco/blob/main/readme.md)、[C++ bridge](https://github.com/unitreerobotics/unitree_mujoco/blob/main/simulate/src/unitree_sdk2_bridge.h)。
2. [Isaac Gym 论文](https://arxiv.org/abs/2108.10470)、[legged_gym](https://github.com/leggedrobotics/legged_gym)。
3. [云深处 rl_training](https://github.com/DeepRoboticsLab/rl_training)、[MJX 官方文档源码](https://github.com/google-deepmind/mujoco/blob/main/doc/mjx.rst)。
4. [Unitree legacy 帧定义](https://github.com/unitreerobotics/unitree_legged_sdk/blob/master/include/unitree_legged_sdk/comm.h)、[UDP 示例](https://github.com/unitreerobotics/unitree_legged_sdk/blob/master/example/example_position.cpp)。
5. [Unitree SDK2 Go2 低层例程](https://github.com/unitreerobotics/unitree_sdk2/blob/main/example/go2/go2_low_level.cpp)、[Actuator SDK](https://github.com/unitreerobotics/unitree_actuator_sdk)。
6. [Lite3 MotionSDK](https://github.com/DeepRoboticsLab/Lite3_MotionSDK/blob/main/README_ZH.md)、[UDP Sender](https://github.com/DeepRoboticsLab/Lite3_MotionSDK/blob/main/include/sender.h)。
7. [Deep Motor SDK](https://github.com/DeepRoboticsLab/Deep_Motor_SDK/blob/main/README_ZH.md)、[CAN 定义](https://github.com/DeepRoboticsLab/Deep_Motor_SDK/blob/main/sdk/can_protocol.h)。
8. [Fast DDS Data-sharing](https://github.com/eProsima/Fast-DDS-docs/blob/master/docs/fastdds/transport/datasharing.rst)、[iceoryx2](https://github.com/eclipse-iceoryx/iceoryx2)。
