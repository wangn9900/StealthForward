# StealthForward 系统优化记录与路线图

## 📍 当前版本里程碑: v3.6.68 (2026-01-08)

### ✅ 已完成的核心改进
1.  **流量持久化 (Traffic Persistence)**
    *   机制：内存全量累加 + 数据库增量更新 (Incremental Update)。
    *   效果：彻底解决 Controller 重启后流量统计回退/清零的问题。
    *   细节：实现了 `syncedEntryTraffic` 游标机制，确保数据库只写入差值，并发安全。

2.  **流量上报回滚 (Traffic Rollback)**
    *   机制：在向 V2Board 上报流量失败时，自动将已提取的流量原子加回内存计数器。
    *   效果：防止因网络波动或 V2Board 维护导致的用户流量“白嫖”/数据丢失。
    *   保障：实现“1KB 不拉”的闭环统计。

3.  **UI 交互增强**
    *   新增：入口/落地节点卡片上的“清除流量”按钮（悬停可见），方便运维重置。

4.  **Shadowsocks 连接性优化**
    *   配置：恢复了 `tcp_keep_alive_interval` (15s) 和 `tcp_multi_path`。
    *   修复：移除了导致 Windows 编译失败的 `rlimit` 代码。

---

## 🚦 性能瓶颈诊断 (Current Bottlenecks)

### 1. Controller CPU 占用偏高 (~40-60%)
**现象**：
`stealth-controller` 进程在同步 V2Board 用户时 CPU 飙升。
**原因分析**：
`internal/sync/v2board.go` 中的 `updateRulesForEntry` 采用了**逐条处理**模式：
*   假设有 30 个节点映射，每个节点 100 用户。
*   每 2 分钟触发一次全量同步。
*   系统必须执行 `30 * 100 = 3000` 次数据库查询 (`SELECT`) + `3000` 次数据库写入 (`UPDATE/INSERT`)。
*   大量短小的 SQL I/O 操作和 ORM 反射开销导致 CPU 居高不下。

### 2. 节点切换瞬时流量遗漏 (Edge Case)
**现象**：
用户自动切换节点的一瞬间（约 1 分钟窗口期），流量可能未被统计。
**原因分析**：
Controller 同步逻辑是先删旧规则再加新规则。如果 Agent 在规则被删的空窗期上报了旧节点的流量，Controller 会因为找不到 owner 而丢弃。

---

## 🚀 未来优化路线图 (Roadmap)

### 🛠️ 优先级：高 (High Priority) - 解决 CPU 发热
**[优化] 数据库批量事务 (Batch Transaction)**
*   **方案**：重构 `internal/sync/v2board.go`。
*   **逻辑**：
    ```go
    database.DB.Transaction(func(tx *gorm.DB) error {
        // 在一个事务内完成该节点所有用户的增删改
        // 将 100 次 IO 合并为 1 次 commit
        return nil
    })
    ```
*   **预期效果**：IOPS 降低 90%，CPU 占用率降低至 5% 以下。

### 🛠️ 优先级：中 (Medium Priority) - 极致数据准确性
**[优化] 流量无损漫游 (Lossless Roaming)**
*   **方案**：引入 Soft Delete (软删除) 机制。
*   **逻辑**：
    1.  修改 `ForwardingRule` 模型，增加 `ArchivedAt` 字段。
    2.  用户切走时，不物理删除规则，而是标记为 `Archived`。
    3.  流量上报时，允许匹配 1 小时内的 Archived 规则。
    4.  每日定时清理过期规则。
*   **预期效果**：彻底覆盖“自动切换节点”场景下的流量统计。

**[优化] 极端容灾 (Extreme Disaster Recovery)**
*   **方案**：Controller 本地流量 Dump。
*   **逻辑**：在 Controller 优雅退出或定时（每小时）将 `userTrafficMap` (待上报 V2Board 的增量) 序列化到磁盘 Json 文件。重启时加载。
*   **预期效果**：即使 V2Board 挂了且 Controller 随后也挂了，中间积压的流量也能在重启后找回。

---
**维护者**: Antigravity
**最后更新**: 2026-01-08
