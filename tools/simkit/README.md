# 78QuadLink / simkit — v0.5 Go 工具包

目标是可选小工具 + 机器人适配层，不是新仿真平台。独立 Go module，标准库依赖，
Linux 上构建；当前实际验证为 Ubuntu 22.04 amd64、Go 1.18。不需要 ROS 来编译或分析归档。
源码目录可单独构建 `go build .`；包含 Go1 后端的打包脚本需要本仓库相邻 go_relay 源码。

## 能力与边界

- `doctor`：严格读取 JSON，检查配置和后端可执行文件，展示所选 profile。
  不启动仿真，不宣称匹配了实际加载的 ABI 或验证了机器人兼容性。
- `run`：独占临时共享板、清除其他 profile 遗留环境变量、启动传入命令、传递信号、退出清理。
  不要求修改传入项目的启动脚本；**该项目本身必须已有匹配的共享内存适配器**。
- `inspect / record / verify`：通过显式配置的后端执行只读快照、按需压缩录制、完整性核验。
  当前后端 CLI 合约是 `go1relay-flags-v1`，不是任意已有程序都能直接接入。
- `analyze`：Go 独立流式分析 G1REC1 归档，核验 gzip CRC、逐记录及整文件 SHA-256、
  数量/大小；按分区统计数据量、提交计数间隔和重复/回退。内存不随归档长度增长。
  无需后端、ROS、Gazebo、机器人模型；不解析电机语义，不推断控制延迟。

提供已验证的 Go1 与 A1 Gazebo Classic adapter，以及未通过运动检查的 Go2 实验 adapter。A1 使用真实官方模型及独立运动学控制程序。
JSON 抽离的是选择配置与集成合约，不能靠改 robot 名称适配另一种电机协议。
Go1 电机数据面仍是十二关节格式；现有 C++ source/consumer adapter 仍必需。
独立工具包不安装 ROS/Gazebo 或常驻服务；完整部署入口为仓库根目录 `./deploy.sh`。
`prepare-robot -repo external/go1sim -robot a1 -output NEW_DIRECTORY` 在启动前检查关节/限位，
解析 mesh 并生成 A1 ROS 2 模型及带 SHA-256 的 contract；不在每帧路径中做转换。

## 构建与打包

在仓库根目录，使用 Go 1.18 或更新版本：

```bash
# 需要 Go >= 1.18；完整部署会检查工具链。
./tools/simkit/package.sh /tmp/78quadlink-tools-v0.5
cd /tmp/78quadlink-tools-v0.5
sha256sum -c SHA256SUMS
./bin/simkit version
./bin/simkit doctor -adapter adapters/go1-gazebo-classic.json -backend ./bin/go1relay
```

输出目录必须不存在；目录可整体搬迁，二进制不依赖 ROS 或 C 动态运行库。
SHA256SUMS 可检测复制损坏，不是发布者身份签名。移除目录即可移除工具包。

仓库内可执行 `python3 tools/simkit/check_bundle.py /tmp/78quadlink-tools-v0.5`
检查包的哈希、基本命令和退出清理。可加 `--archive PATH.gz` 验证已有真实录制。

## 使用

在已构建并 source ROS/机器人 overlay 的终端，用绝对路径替换以下 KIT、REPO：

```bash
KIT=/tmp/78quadlink-tools-v0.5
REPO=/absolute/path/to/78QuadLink/external/go1sim
"$KIT/bin/simkit" run \
  -adapter "$KIT/adapters/go1-gazebo-classic.json" \
  -backend "$KIT/bin/go1relay" -profile motor-imu -- \
  "$REPO/gazebo_ros2/start_go1_sim_with_ctrl.sh" gui:=false use_rviz:=false
```

可用 profile：`motor`、`motor-imu`、`motor-imu-odom-experimental`。
推荐 motor-imu；odom 在 v0.4.2 对照中未进一步省 CPU，不默认打开。
所有参数位于 `--` 之前，后面是原样传递的命令参数，不经 shell 插值。
`run` 是每次实验的可选父进程，等待退出时没有轮询；不是系统常驻服务。
启动日志输出本次 board 路径。在另一终端按需执行：

```bash
"$KIT/bin/simkit" record -adapter "$KIT/adapters/go1-gazebo-classic.json" \
  -backend "$KIT/bin/go1relay" -board /dev/shm/simkit-EXAMPLE/state \
  -output /tmp/experiment.gz -duration 8s -period 2ms
"$KIT/bin/simkit" analyze -input /tmp/experiment.gz > /tmp/experiment-analysis.json
```

录制期间仿真需保持运行。`record` 不覆盖已有文件；归档与 `.json` manifest 一起保存。
采样的是最新状态，不保证每个更新都被录下；revision gap 是未观察到的提交，不是网络丢包。
跨分区不是同一 tick 的原子快照。manifest 的丢弃计数是 recorder 报告的数据；
对 manifest 的编辑没有独立身份认证，分析器不会把它们称为自己测得的丢包率。

退出时向本次子进程组转发 INT/TERM，最多等待 3 秒，然后清理同组剩余进程。
工作负载不应脱离进程组自行 daemonize；像 long_bench 这样自行建立额外进程组的程序
须负责自己的子进程清理。此工具不提供整机进程管理或 cgroup 容器隔离。
不使用 simkit 时，原启动方式不变；要完全退回 ROS 路径，另需取消手动导出的 GO1SIM 模式变量。

## 增加另一种机器狗

1. 明确 producer/consumer、关节顺序、单位、时间、有效期和帧 schema。
2. 提供匹配的源头/消费端 adapter；按现有 backend CLI 合约提供初始化、读取和录制程序，
   或先扩展并测试新的 backend 合约。不能只改 JSON 绕过数据格式限制。
3. 添加 JSON（schema=1），声明 board ABI、payload schema、环境变量和可选 profile。
4. 进行原路径/新路径同输入功能对照，最后测性能，再声明该机器人受支持。

v0.5 已验证 Go1/A1 的固定十二关节适配。Go2 已试测但运动验证未通过；B2、任意关节数和其他引擎未验证。
独立 tools 包不包含模型；prepare-robot 需用 -repo 指向完整源码/完整发布包。
完整功能/性能及部署验证见 [v0.5 报告](https://github.com/NHK-DOT/78QuadLink/blob/v0.5/docs/v05_release.md)。
