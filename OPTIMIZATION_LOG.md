# StealthForward 系统优化记录与路线图

## 📍 当前版本里程碑: v3.6.72 (2026-01-08)

### ✅ 已完成的核心改进
1.  **[v3.6.72] 二进制深度混淆 (Binary Obfuscation)**
    *   **机制**：构建参数增加 `-trimpath`，移除所有源文件路径信息。
    *   **效果**：提高逆向工程门槛，防止内部目录结构泄露，提升安全性。
    *   **状态**：已集成至 default build workflow。

2.  **[v3.6.71] 公私有库分离战略 (Architecture Planning)**
    *   **成果**：完成了 "Core (私有) + Release (公有)" 的全套 CI/CD 脚本设计。
    *   **状态**：脚本已备份至 `.github/workflows/build_public_release.yml.inactive`，随时可启用。主分支代码已强制同步。

3.  **[v3.6.70] 极致同步性能优化 (Ultra-Fast Sync)**
    *   **机制**：全量预加载规则 + 内存 Map O(1) 比对 + 数据库事务。
    *   **效果**：将 N 次 SQL 查询降低为 1 次。CPU 峰值从 >90% (卡顿) 降低至 ~50% (瞬间流畅)。
    *   **详情**：彻底消灭了 `updateRulesForEntry` 中的 N+1 查询问题。

4.  **[v3.6.69] 流量已修复 (Traffic Fix)**
    *   **修复**：启动时自动清洗数据库中的 NULL 流量字段为 0。
    *   **效果**：解决了增量更新失效导致 UI 流量显示“自动清零/一直很少”的 Bug。

---

## 🚦 观察中的性能指标

### Controller CPU 占用
*   **优化前**: >85% (高 I/O Wait，导致 SSH 卡顿)
*   **优化后**: ~50% (纯计算峰值，JSON 解析与内存比对，无 I/O 阻塞)
*   **状态**: **健康**。50% 是 Go 运行时处理网络数据和 GC 的正常开销，耗时极短。

### 网络连通性 (AWS -> DMIT)
*   **现象**: AWS 节点连接特定 DMIT 落地机出现大量 `i/o timeout`。
*   **诊断**: 双向 TCP 抓包显示网络层畅通 (Ping OK)，但 TCP 握手在大流量或特定时刻被丢包。
*   **结论**: 判定为特定线路的基础设施问题（回程路由拥堵/运营商干扰/MTU主要原因）。
*   **建议**: 开启 BBR，调低 MTU (1380)，或更换落地节点。

---

## 🚀 未来优化路线图 (Roadmap)

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
