# 共享板 SDK 与数据格式

Linux GCC/Clang C++17，要求原生 64 位无锁原子操作；当前验证 amd64、小端。头文件保留 `simulation_state` / `go1sim_relay` 命名空间以兼容既有适配器。

共享板 8768 B，8 个分区、每区最多 1024 B payload。`Board::open(path)` 为只读；`open(path, slot)` 申请该分区唯一写者，Linux OFD 文件锁排除另一合规写者。各进程映射到自己的虚拟地址，底层物理页共享，不要求虚拟地址相同。

`publish(data, size)` 提交分区；`snapshot(slot, buffer, capacity, size, revision)` 返回一致快照或失败，不保证每轮必定读取成功。写者标记奇数 revision，原子写入长度/数据，再提交偶数 revision；读者复制后复核 revision，变化则拒绝。所有消费者应处理 false、停更与超时。各分区独立，无跨区全局事务；电机状态与 IMU 时间戳可不同。示例仅示范 IPC，不代替控制器 watchdog。

| 分区 | 当前集成用途 | 帧 |
|---|---|---|
| 0 | 电机命令 | fixed12-v2，328 B |
| 1 | 电机状态 | fixed12-v2，280 B |
| 2 | IMU | OBS1，四元数/角速度/加速度及时间戳 |
| 3 | 可选单条 TF 边 | OBS1，不是完整 TF 树 |
| 4 | 可选 odom 实验 | ODO1，844 B，默认关闭 |
| 5–7 | 预留 | 自行定义契约 |

电机协议固定 FR/FL/RR/RL × hip/thigh/calf，rad、rad/s、N·m。显式字段编码，不传 C++ 指针或直接 memcpy 原生对象；帧带版本/类型/长度与 CRC32C。版本核对确认解释规则，CRC 检查帧，SHA-256 检查模型/归档内容与固定集成提交，三者用途不同，哈希不能替代 ABI 版本。

`go1relay -init-board PATH` 初始化板；具体 C++ 调用见 `examples/board_roundtrip.cpp`。板读者只能取得最新快照，没有队列积压；需要历史必须主动录制。Go 检查/录制以只读映射访问，不占据控制槽。

文件权限、独占临时目录和单写者锁防止误接入。OFD 锁是协作规则，不阻止拥有写权限的恶意进程直接 mmap 覆写；CRC/SHA 也不是授权机制。不要向不受信任进程开放控制板写权限。

库不接管仿真器时钟、接触求解或传感器渲染。其他机器人需要实现原有控制框架到该契约的 adapter，并验证关节顺序、单位、坐标系、刷新率和停更处理。新协议应显式增加 schema，不能只改机器人名字。
