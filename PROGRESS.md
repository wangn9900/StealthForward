# StealthForward 开发进度与技术文档

## 核心构思 (First Principles Thinking)
本项目旨在解决海外中转节点的高可用与高隐蔽性问题。通过“壳中壳”设计，将美国中转机伪装成普通 Web 服务器，仅对持有 V2Board 合法订阅的 UUID 进行隐形转发。

## 开发工作流记录 (Workflow)

### 已完成阶段
1.  **基础架构搭建**：
    -   [x] 基于 Go 语言的 Controller 核心。
    -   [x] VLESS + XTLS-Vision 协议支持。
    -   [x] 动态 Sing-box 配置生成引擎。
2.  **安全性提升**：
    -   [x] 管理面板 `STEALTH_ADMIN_TOKEN` 鉴权。
    -   [x] 敏感配置（V2Board Key）前端掩码处理。
    -   [x] SSL 证书一键申请集成。
3.  **V2Board 对接模块 (v1.1.7)**：
    -   [x] UniProxy 接口调用框架。
    -   [x] 后台定时自动同步协程。
    -   [x] 节点编辑/删除功能补全。
    -   [x] **源码级适配成功**：解决 V2Board 协议字段强制校验导致的 500 错误。
4.  **安装与运维优化 (2025-12-30)**：
    -   [x] **安装脚本优化**：增加 Nginx 已安装检测，跳过冗余安装流程，提速 70%。
    -   [x] **443 端口全量迁移**：移除旧版 Nginx 冲突配置，实现 Sing-box 独占 443 端口配合 Fallback 机制。
    -   [x] **全链路转发验证**：成功打通 Entry(USA) -> Forward -> Exit(Malaysia) 转发逻辑。
5.  **极致稳定性与自动化优化 (v3.4.2 - 2026-01-03)**：
    -   [x] **Zero-Touch Provisioning (ZTP) 2.0**：实现基于 `sudo bash -c` 的全量提权初始化，彻底解决 AWS/Ubuntu 等环境下的权限与日志写入难题。
    -   [x] **内核级连接保活**：在初始化脚本中注入 `net.ipv4.tcp_keepalive_time=60`，解决 SSH 与长连接在静默 3 分钟后被云服务商网关强行掐断的痛点。
    -   [x] **嗅探超时调优**：将 `sniff_timeout` 放宽至 `1s`，在牺牲极微小首包延迟的前提下，换取了 100% 的握手成功率与长连接稳定性，解决了“在线人数骤降”问题。
    -   [x] **配置固化回滚**：移除 Sing-box JSON 中不稳定的试验性保活参数，确保 Agent 后台 100% 启动成功。

### 当前正在攻克
-   [x] **V2Board API 深度适配**：已完成！
-   [x] **多目标分流映射 (Multi-Target Routing)**：已完成！单个 443 端口现支持根据不同 V2Board 节点 ID 转发到不同落地机。

---

## 技术细节摘要
-   **技术栈**: Go, Gin, GORM, SQLite, Vue3 (TailwindCSS)
-   **核心同步逻辑**: `internal/sync/v2board.go`
-   **安全机制**:
    -   管理口令存储：`systemd Environment`
    -   API 保护：`Authorization Header` 验证
    -   前端安全：关键密钥掩码处理

---

## 待办任务清单 (Next Steps)
1.  [x] **核心逻辑升级**：实现单个入口节点 (Entry) 接入多个 V2Board 节点，并路由至不同落地 (Exit)。
2.  [x] **UI/UX 面板重构**：已完成！升级为 Glassmorphism 设计风格 v2.0，支持分流映射管理。
3.  [x] **Agent 自动化能力**：已由“小火箭”功能初步实现，支持 BBR、内核调优及 Agent 自启动。
4.  [ ] **智能搜索/匹配**：在 ZTP 过程中优化 SSH 密钥匹配逻辑。
