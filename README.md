[黑白像素 U / 闪电 ICO 图标](assets/quadlink78.ico)

# 78QuadLink · v0.5.1

**78big 的四足仿真共享状态库与 Go 工具包。** 将高频关节/IMU 数据接入固定布局共享内存，把模型适配、检查、按需录制、压缩和分析交给 Go。

这是从 [go1sim](https://github.com/NHK-DOT/go1sim) 拆出的独立库。核心目录没有 Gazebo 工程和完整机器人模型；不依赖 ROS 即可编译 SDK、运行 Go 工具和分析归档。完整演示通过 [integration.lock.json](integration.lock.json) 固定集成仓库的 **Git 提交哈希**，并核对 SDK 与集成端四个头文件的 SHA-256，保留原仿真和控制器。

## 实测优化

Go1/A1 数字来自 v0.5，Go2 来自 v0.5.1。各自使用同一台机器、同一机器人场景、GUI 关闭、固定站立；预热 10 秒，每轮采样 45 秒，计算全部被测进程组及 Go 包装进程。CPU 100% 表示一个逻辑核。

| 模型 | 原十二路 CPU | 共享电机 + IMU CPU | CPU 降幅 | RSS 减少 | 实时倍率 |
|---|---:|---:|---:|---:|---:|
| Unitree A1（两轮均值） | 98.93% | 41.96% | **57.6%** | 20.4 MiB | 1.0 |
| Unitree Go1（一轮） | 100.11% | 43.58% | **56.5%** | 22.1 MiB | 1.0 |
| Unitree Go2（v0.5.1，两轮均值） | 96.19% | 39.57% | **58.9%** | 21.4 MiB | 1.0 |

[各轮数据](docs/v05_measurements.json) · [兼容性与验证报告](docs/v05_release.md) · [历史性能](docs/v043_performance.md) · [现有行业方案](docs/quadruped_optimization_landscape.md)

**v0.5.1 新增 Go2 验证：** 修复原仿真关节增益过大导致的失稳；共享/十二路均通过 30 秒连续行走。IMU 匹配 64,610 次、TF 6,585 次，无不一致；[原因与证据](docs/go2_tuning_v051.md)。

A1 使用宇树官方 A1 模型和 A1 运动学，启动前适配 ROS 1 模型接口，保留物理参数。A1 的 IMU 同时间戳核对 43,215 次、TF 边核对 4,386 次，无不一致；两种狗均通过站立→短时小跑→站立检查（前进约 0.82/0.83 m）。这不是长期复杂地形或真机认证，也不意味着控制延迟、训练速度、功耗下降同样比例。

历史最初十二路到聚合 DDS 的主要进程 CPU 为 93.81%→51.74%（降低 44.8%）；统计范围与本轮不同，不能把百分比相加。共享内存、聚合帧是成熟技术，本项目的价值是可选接入、工具化和可复现收益。

## 一键部署与运行

完整演示验证环境：**Ubuntu 22.04 / ROS 2 Humble / Gazebo Classic / Linux amd64**。首次部署需要网络；缺少系统依赖时脚本通过 sudo 安装，已有依赖则跳过。

```bash
git clone --branch v0.5.1 --depth 1 https://github.com/NHK-DOT/78QuadLink.git
cd 78QuadLink
./deploy.sh
~/.local/bin/quadlink78 go2 --gui
# 或 ~/.local/bin/quadlink78 a1 --gui
# 或 ~/.local/bin/quadlink78 go1 --gui
```

按 `2` 站立，`4` 小跑，`W/S/A/D` 调整速度，`2` 返回站立。省略 `--gui` 为无显示运行。默认共享电机 + IMU；`--dds` 回退聚合 DDS，`--legacy` 回退十二路模式。`./deploy.sh --jobs 4` 调整编译并行度。

只需要 SDK / Go 工具：

```bash
./deploy.sh --tools-only
~/.local/bin/quadlink78 version
./artifacts/bin/simkit doctor -adapter tools/simkit/adapters/a1-gazebo-classic.json -backend ./artifacts/bin/go1relay
```

工具安装需已有 Go >=1.18、CMake、C++17 编译器；无需 ROS。SDK 安装到 `artifacts/sdk`。入口是符号链接，保留源码目录，搬迁后重新部署。卸载时删除 `~/.local/bin/quadlink78` 和此目录；系统 ROS/Gazebo 依赖不自动卸载。

## 作为库使用

```cmake
find_package(quadlink78 0.5 CONFIG REQUIRED)
target_link_libraries(my_adapter PRIVATE quadlink78::board)
```

设置 `-DCMAKE_PREFIX_PATH=/path/to/78QuadLink/artifacts/sdk`。
[跨进程示例](examples/board_roundtrip.cpp) 可独立运行：

```bash
cmake -S . -B build
cmake --build build
ctest --test-dir build --output-on-failure
```

[接口与共享内存设计](docs/library.md) 描述分区、原子快照、锁与协议边界。库是 header-only C++17；Go CLI 为独立 module、仅依赖标准库。现阶段 Go 接口是 CLI，尚未承诺稳定的可导入 Go package API。

## Go 的职责

- 启动前：解析 A1 URDF，核对十二关节映射/限位，解析模型资源，生成 ROS 2 模型与 SHA-256 契约。
- 运行时：按 profile 创建共享板、启动进程、交接终端、传递信号、退出清理；不在每个电机帧之间转发。
- 按需：只读检查、压缩录制、归档哈希核验、流式分析；分析无需 ROS、模型或仿真进程。
- 旧 UDS relay 的注册、续租、转发仍作为可选后端保留；共享模式无需常驻 relay。

历史 recorder 测量约 2.0%–2.3% 单核 CPU、8.3 MiB RSS，归档缩小约 58.4%。压缩用于录制，不放入电机实时路径。录制是最新状态采样，可能跳过更新，不替代无损 rosbag。

[Go 工具说明](tools/simkit/README.md) · [底层后端说明](tools/go_relay/README.md)

## 兼容范围与目录

当前适配范围限**宇树系四足**：已实测 **Go1、A1、Go2**。Go2 在 v0.5.1 修正专用仿真增益后，通过两路径 30 秒连续行走及 IMU/TF 核对（[修复报告](docs/go2_tuning_v051.md)）。B2、其他品牌、MuJoCo、Isaac 尚未在这套插件上实测。核心共享板与机器人无关，现成电机协议固定十二关节，适配 JSON 不能替代真实仿真器/控制器读写接口。ROS 2 仍负责其他 topic、TF 和集成；不是 MQTT/CAN 总线，也没有消除数据读写成本。

| 目录 | 内容 |
|---|---|
| `include/quadlink78` | 无 ROS 依赖的共享板与帧编码头文件 |
| `tools/simkit` | Go 生命周期、模型契约、归档分析与机器人配置 |
| `tools/go_relay` | 板初始化、检查、录制与可选 UDS relay |
| `external/go1sim` | 部署时拉取的固定提交集成，Git 忽略 |
| `testdata` | 官方 A1 URDF 测试样本及原许可证 |
| `assets` | 16 / 32 / 48 px 黑白像素 ICO 图标 |
| `docs` | ABI、实测结果、历史对比与来源 |

C++ 继续承担 Gazebo/ros2_control 边界与控制算法；不为语言占比改写热路径。共享 odom 实验未证明进一步收益，默认关闭；TF 保持正常 ROS 消费兼容。

## 发布与许可

[GitHub v0.5.1 Release](https://github.com/NHK-DOT/78QuadLink/releases/tag/v0.5.1) 提供核心源码、Linux amd64 工具包、图标与 SHA256SUMS。完整仿真通过固定提交依赖部署，首次安装仍需网络，不把工具包称为离线 ROS 安装包。

项目代码采用 [BSD-3-Clause](LICENSE)；第三方模型样本与外部集成遵循各自原许可，详见 [THIRD_PARTY.md](THIRD_PARTY.md)。
