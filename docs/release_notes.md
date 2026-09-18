# 78QuadLink v0.5

独立发布的宇树系四足仿真共享状态库与 Go 工具包。核心不包含完整 Gazebo 工程，通过固定 Git 提交和 SDK 头文件 SHA-256 核对接入 go1sim。

- 已验证 Go1、A1：短窗口整组 CPU 分别降低约 56.5%、57.6%，RTF=1；均通过短时站立/小跑回归。详情与每轮数据见 README/docs，不承诺其他场景固定收益。
- Go 扩展为模型解析/契约、进程启动、按需录制压缩、哈希核验、流式分析；实时电机路径保持直接共享快照。
- 无 ROS 依赖的 C++17 header-only SDK、CMake package、跨进程示例、Go 标准库工具。
- 一键部署、SVG/PNG/ICO 图标、源码和 Linux amd64 工具包、SHA256SUMS。
- Go2 已使用官方模型试测：十二路/共享两条路径均未通过运动检查，仍为实验状态，默认拒绝启动，不计入兼容名单和性能宣传。

完整部署：解压源码后 `./deploy.sh`，随后 `~/.local/bin/quadlink78 a1 --gui` 或 `go1 --gui`。Ubuntu 22.04 / ROS 2 Humble / Gazebo Classic；首次安装需网络，缺依赖时需 sudo。

工具包可独立运行，无需 ROS；它不是离线 ROS/Gazebo 安装包。下载后用 `sha256sum -c SHA256SUMS` 核验。源码部署使用的完整集成提交记录在 integration.lock.json。
