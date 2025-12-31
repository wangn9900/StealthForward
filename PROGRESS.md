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
1.  [ ] **核心逻辑升级**：实现单个入口节点 (Entry) 接入多个 V2Board 节点，并路由至不通落地 (Exit)。
2.  [x] **UI/UX 面板重构**：已完成！升级为 Glassmorphism 设计风格 v2.0，支持分流映射管理。
3.  [ ] **Agent 自动化能力**：支持 Agent 端自动申请/同步 SSL 证书，解决跨机房部署时的证书手动搬运痛点。
4.  [ ] **实时监控模块**：增加流量镜像与并发连接数实时曲线展示。
