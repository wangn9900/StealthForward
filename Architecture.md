# Architecture & Design Blueprint

## 1. 核心业务流程
`用户` -> (VLESS + XTLS-Vision) -> `入口节点 (StealthForward Agent)` -> (内部流转/解密) -> `路由决策` -> (Outbound 协议) -> `落地节点 (SS/VLESS/etc.)`

## 2. 关键组件
### Controller (控制面)
- **管理端**：处理逻辑、用户分配、节点下发。
- **存储**：记录节点信息、规则映射、流量统计。

### Agent (执行面)
- **内核控制**：动态管理 Sing-box 实例。
- **监控上报**：实时汇报节点状态。

### Routing Engine (路由引擎)
- 基于 Sing-box 的 Routing 模块进行定制。
- 实现 User -> Exit Node 的映射。

## 3. 防护策略
- **DPI 规避**：强制执行 XTLS-Vision 流控。
- **指纹模拟**：Reality/Fallback 机制。
- **行为混淆**：支持定时的伪装站内容更新。

## 4. 转发链表示例
我们将转发链路模型抽象为：
`[Entry: HK-01] -- (User: Alice) --> [Routing: Relay to JP-SS] -- (Outbound: SS) --> [Exit: JP-Node]`
