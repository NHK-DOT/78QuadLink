# 78QuadLink v0.5.1

修复 Go2 从 A1/Go1 沿用高 PD 增益导致的仿真失稳，保留真实模型、力矩限位和 1 ms 仿真步长。只调整 Go2 的站立与 passive 关节增益，共享内存协议及 Go1/A1 参数保持兼容。

- Go2 十二路/共享路径均通过连续 30 秒行走与返回站立检查，另有逐段姿态采样。
- Go2 IMU 同时间戳匹配 64,610 次、TF 匹配 6,585 次，无不一致；普通 ROS TF 查询正常。
- Go2 同模型两轮性能对照与完整证据见 README、docs/go2_tuning_v051.md。
- 支持直接 `quadlink78 go2 --gui`；当前实测范围为宇树 Go1、A1、Go2。
- 图标更换为宇树官网 U 轮廓的黑白像素化版本，加小闪电；仅保留 16/32/48 px ICO，无 PNG/SVG 图标文件。

部署：源码包解压后执行 `./deploy.sh`。Ubuntu 22.04 / ROS 2 Humble / Gazebo Classic；首次安装需网络，缺依赖时需 sudo。Go 工具可单独部署 `./deploy.sh --tools-only`。

这次验证针对当前平地场景，不等于真机、高速或复杂地形验证；旧 v0.5 失败证据仍保留。图标借鉴的 Unitree U 商标归宇树所有，本项目并非宇树官方软件。
