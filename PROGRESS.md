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
3.  **V2Board 对接模块 (v1.1.x)**：
    -   [x] UniProxy 接口调用框架。
    -   [x] 后台定时自动同步协程。
    -   [x] 节点编辑/删除功能补全。

### 当前正在攻克
-   [ ] **V2Board API 深度适配**：解决特定版本下 `node_id` 请求报 500 错误的问题。
-   [ ] **增强型日志诊断**：捕获 V2Board 返回的原始错误 Body。

---

## 技术细节摘要
-   **技术栈**: Go, Gin, GORM, SQLite, Vue3
-   **核心同步逻辑**: `internal/sync/v2board.go`
-   **安全机制**:
    -   管理口令存储：`systemd Environment`
    -   API 保护：`Authorization Header` 验证

---

## 待办任务清单 (Next Steps)
1.  [ ] 分析 V2Board 源码，明确 UniProxy 接口的准确入参与逻辑。
2.  [ ] 根据源码修正 `fetchUsersFromV2Board` 函数。
3.  [ ] 增加入口节点连接数统计 UI 模块。
