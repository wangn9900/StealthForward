# StealthForward 开发进度日报 (2026-01-02)

## 今日完成事项 (Done)

### 1. 核心功能：AWS IP 轮换 (Auto Heal)
- **目标**：实现一键更换被墙 IP，无需重建实例。
- **实现细节**：
    - 集成 `aws-sdk-go-v2`，实现申请新 EIP、强制绑定到实例、释放旧 IP 的全流程。
    - 集成 `cloudflare-go`，实现自动更新 DNS A 记录。
    - 新增 API 接口 `POST /api/v1/node/:id/rotate-ip`。
    - 修复了 AWS SDK 字段名称 (`AllocationID`) 和 Cloudflare 指针参数的编译错误。
- **状态**：已上线 (v2.2.0+)。

### 2. 系统架构：配置与安全管理
- **目标**：解决敏感信息（AK/SK）硬编码问题，增加系统安全性。
- **实现细节**：
    - **数据库**：新增 `SystemSetting` 表，存储 AWS AccessKey、SecretKey、DefaulRegion 及 Cloudflare Token。
    - **前端 UI**：新增 "系统 (System)" 选项卡，提供可视化的配置管理界面。
    - **安全认证**：
        - 实现了全局登录拦截（Login Modal）。
        - 默认密码回退机制：环境变量 > 数据库 > 默认值 (`wnazh2006jj`)。
        - UI 优化：增加了“退出 (Logout)”按钮，修复了令牌失效时的空白界面问题。

### 3. 版本发布
- **v2.2.0**：包含 IP 轮换与系统设置功能的首个版本。
- **v2.2.1**：快速修复版本，包含默认密码修正与 UI 退出按钮优化。
- **状态**：已推送到 GitHub，GitHub Actions 自动构建中。

---

## 遗留问题与待办事项 (To-Do)

### 1. 实例一键创建 (Instance Provisioning)
- **优先级**：高
- **描述**：通过 Go 后端直接调用 AWS API 创建新实例（集成原 `aws_create.py` 逻辑）。
- **状态**：已完成 (v2.3.0)。
- **实现细节**：
    - 新增 `internal/cloud/instance.go` 实现自动搜索 Debian 12 AMI、注入 UserData (Root登录)、创建全放行安全组。
    - 前端新增 "新建云端节点" 向导弹窗。

### 2. UI 深度汉化 (Localization)
- **优先级**：高 (用户体验)
- **描述**：当前界面存在中英文混杂，特别是“系统”配置页和“登录”弹窗。
- **具体任务**：
    - [ ] **登录弹窗**：`Admin Password` -> `管理员密码`，`UNLOCK CONSOLE` -> `解锁控制台` 等。
    - [ ] **系统设置页**：`AWS Credentials` -> `AWS 凭证配置`，`Access Key ID` -> `访问密钥 ID` 等。
    - [ ] **全局提示**：确保所有弹窗、底部版权、状态提示均为中文。

### 3. 流量统计与监控
- **优先级**：中
- **描述**：主页显示入口机总流量，分流节点显示单端口流量。
- **状态**：已完成 (v2.3.0)。
- **实现细节**：
    - 后端 `internal/sync/traffic.go` 新增 `totalTrafficMap` 维护全生命周期流量统计。
    - 新增 `/api/v1/traffic` 接口供前端获取实时数据。
    - 前端在 Entry 卡片和 Mapping 列表中展示流量数据。

### 4. 系统健壮性
- **优先级**：低
- **描述**：IP 轮换过程目前是同步调用，等待时间较长（3-5秒）。
- **下一步动作**：考虑改为异步任务（Task Queue），前端显示进度条。

---

## 部署说明 (针对 v2.2.1)

1. **拉取更新**：
   ```bash
   # 在服务器执行
   ./update.sh # 或手动拉取二进制文件
   ```

2. **登录系统**：
   - 访问 Web 界面。
   - 密码：若未设置环境变量，使用默认密码 `wnazh2006jj`。

3. **初始化配置**：
   - 进入“系统”标签页。
   - 填入以下 AWS 凭证 (请妥善保管)：
     - **AccessKey ID**: `AKIAXU2L4Q55R7R3****` (示例，请替换真实AK)
     - **Secret Access Key**: `w4S8F...` (示例，请替换真实SK)
   - **Default Region**: `ap-northeast-1` (东京)
   - 填入 Cloudflare API Token。
   - 保存配置。

4. **测试**：
   - 在“概览”页选择一个 AWS 节点，点击“Rotate IP”图标测试换 IP 功能。
