# 78QuadLink v0.5：Go1 + A1 兼容性与部署（Go2 实验未通过）

78QuadLink（78big 风格的四足共享状态连接工具）由 go1sim 演进而来。
它是可选优化插件和外置工具包，不是重写 Gazebo、ROS 2 或所有运动控制器。
独立库发布在 [NHK-DOT/78QuadLink](https://github.com/NHK-DOT/78QuadLink)，
外部 go1sim 仓库保留模型、控制器和仿真集成，本库按提交哈希固定引用。

## 第二种机器人：宇树 A1

采用 Unitree 官方 `unitree_ros` 的真实 A1 URDF/网格，固定源提交
`ccfc6fd8430a17ba3dacef9a1e2faf64ff3b0aee`，不是改名后的 Go1。
保留 BSD-3-Clause 授权、模型来源、惯量、碰撞几何及关节限位。
A1 以已有 A1Robot/A1Leg 和对应编译参数独立构建 `junior_ctrl_a1`，Go1 仍用
`junior_ctrl`，不以 Go1 几何冒充 A1 运动学。

发现并修复的接口不兼容：

- 上游 A1 使用 ROS 1 插件/transmission，不能直接加载到 ROS 2；Go 在启动前
  生成单独的 ROS 2 URDF，配置已有 effort controller 与 IMU/odom 插件。
- A1 根坐标叫 base，当前消费端使用 base_link；启动前转换引用，保留物理参数。
- 复位逻辑原来写死 GO1 实体，现在读取 SIM78_ENTITY（未设置仍为 GO1）。
- 基准与键盘工作负载原先写死 junior_ctrl/GO1，现允许实际的 A1 进程/实体。
- 二进制接口不是任意自由度：当前明确定义 FR/FL/RR/RL，每条腿 hip/thigh/calf，
  共十二个 revolute joint，单位 rad、rad/s、N·m。不符合契约便拒绝准备，避免默默错配。

## Go 扩大的职责

新增 `prepare-robot`：解析 URDF，检查关节类型、数量、顺序、有限且有序的限位，
解析 mesh，生成模型适配与 contract.json，并记录源模型、输出模型和 mesh 的 SHA-256。
与已有 doctor、按需录制、压缩、完整性核验、流式统计一起，Go 负责适配和数据工具。
**适配检查/转换仅在启动前执行，控制循环没有新增反射、JSON、重排或 Go 转发。**

共享板和 328/280 B 电机帧保持原 ABI，IMU 仍从同一个 Gazebo sensor 写入分区 2。
C++ 继续提供 Gazebo/ros2_control 边界和时间敏感控制算法；未为“Go 占比”而搬动热路径。

## 功能与性能证据

A1 首次共享电机 + IMU 功能检查：
`artifacts/bench/20260919-041742-v05-a1-observations`。
IMU 同时间戳匹配 43215 次、TF 边 4386 次，不一致/无效计数零；
固定站立、只读 Go 检查及普通 TF 查询通过，结束高度约 0.333 m。

A1/Go1 分别采用自己的十二路基线与共享路径进行性能对照；不比较不同机器人模型
的绝对 CPU 来宣称优化。A1 采用官方本体 + IMU/odom，没有复制 Go1 的导航雷达；
A1 两条路径的物理与传感器配置相同。Go1 对照保留原 full 场景。

本轮结果（CPU 单核百分比，所有被测进程组及 simkit 包装进程；预热 10 秒，采样 45 秒）：

| 模型 | 十二路 CPU | 共享电机+IMU CPU | CPU 降幅 | RSS 减少 |
|---|---:|---:|---:|---:|
| A1（两轮均值） | 98.93% | 41.96% | 57.6% | 20.4 MiB |
| Go1（一轮） | 100.11% | 43.58% | 56.5% | 22.1 MiB |

每轮 45 个样本，RTF 均为 1。A1 顺序为基线/共享/共享/基线，Go1 为基线/共享。
原始轮次汇总见 [v05_measurements.json](v05_measurements.json)，被测二进制哈希见 [v05_benchmark_binary_hashes.txt](v05_benchmark_binary_hashes.txt)，主机信息见 [validation_host.json](validation_host.json)。
性能采样后修复了交互终端前台进程组交接，并增加 Go2 实验控制器；没有重写 Go1/A1 的共享数据热路径。
这是短窗口结果，未经长期负载/多机器统计，不承诺普遍固定百分比。
独立运动回归：A1 前进约 0.820 m、Go1 约 0.830 m，均能站立→小跑→返回站立。
模型核对保留 40 个惯量/碰撞块以及关节几何、限位、动力学参数。

## 部署与卸载

支持环境：Ubuntu 22.04、ROS 2 Humble、Gazebo Classic、Linux amd64（本次验证）。
从 GitHub 源码或完整源码发布包运行 `./deploy.sh`：检查/补齐系统依赖，构建 Go 工具、
七个 ROS 包（包含 Go1/A1 控制程序及默认禁用入口的 Go2 实验控制程序），安装 `~/.local/bin/quadlink78`。
首次缺系统依赖时使用 sudo，已满足依赖时不执行 apt。`--jobs N` 控制构建并行度。
工具独立安装可以用 `./deploy.sh --tools-only`；仍需本机已有 Go、CMake/ninja 工具链，
该选项不声称安装了仿真运行环境。

运行 `quadlink78 go1 --gui` 或 `quadlink78 a1 --gui`，按 2 站立，4 进入小跑，W/S/A/D
调整速度，2 回到站立。省略 --gui 为无显示运行，仍接受终端键盘输入。
`--dds` 使用合并 DDS，`--legacy` 使用十二路模式；默认共享电机 + IMU。
A1 界面使用 Gazebo，未提供独立 A1 RViz 配置；Go1 的 RViz 沿用原有配置。

用户级入口是指向解压/克隆目录的符号链接，请保留该目录；搬迁后重新运行 deploy.sh。
删除 `~/.local/bin/quadlink78` 与源码/发布包目录即可移除本工具，系统 ROS/Gazebo
依赖不自动卸载，以免影响其他项目。原 go1sim/go1sim-all 命令没有被改写。

## 边界

这不是通用所有机器狗/所有仿真器兼容声明。当前适配范围限宇树系；Go2 已开始实验但尚未通过运动验证，B2 与云深处未实测。
固定站立和短时小跑检查不替代长期复杂地形测试或真机验证。
CPU 数据按一个逻辑核 100% 计，报告 RTF、采样时长和测试模式，不把 CPU 下降
宣称为控制延迟或真实机器人功耗等比例下降。

独立库部署会校验 integration.lock.json 中的完整提交哈希，并从独立目录构建；不会直接复用旧 go1sim 的 build/install 缓存。
