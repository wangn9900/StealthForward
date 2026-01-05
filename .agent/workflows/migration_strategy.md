---
description: StealthForward 仓库迁移与公私分离编译战略规划
---

# 🚀 StealthForward 仓库迁移与公私分离战略规划

**目标**：构建“私有核心源码 + 公有发行渠道”的双库架构，确保商业源码（特别是授权系统）的安全，同时保持用户端的开源/分发体验。

## 1. 架构总览

*   **🛡️ 私库 (StealthForward-Core)**
    *   **性质**：Private (私有)
    *   **内容**：所有源代码 (Go/Vue)、License Server 源码、核心算法。
    *   **职责**：执行 CI/CD 构建、代码混淆、Artifact 打包。
    *   **安全等级**：最高 (绝不公开)。

*   **🌍 公库 (StealthForward-Release)**
    *   **性质**：Public (公开)
    *   **内容**：
        *   `README.md` (项目介绍与文档)
        *   `install.sh` (安装脚本)
        *   **Releases** (存放二进制文件和 web.zip)
    *   **职责**：面向用户的下载源、Issue 反馈（可选）。
    *   **安全等级**：公开 (不含核心源码)。

---

## 2. 实施步骤 (Tomorrow's Task)

### 第一阶段：环境准备
1.  **创建私库**：将当前的 `StealthForward` 为了私有仓库（或者新建一个 Private 仓库并 Push 代码）。
2.  **创建公库**：新建一个空的 Public 仓库（例如 `StealthForward-Release`）。
3.  **Token 配置**：
    *   生成一个 GitHub PAT (Personal Access Token)，权限需包含 `repo` (读写仓库)。
    *   在私库的 `Settings -> Secrets` 中添加 `RELEASE_TOKEN`，填入该 PAT。这是私库向公库发包的“钥匙”。

### 第二阶段：CI/CD 改造 (关键)
修改私库中的 `.github/workflows/build.yml`：
1.  **构建源**：依然在私库运行。
2.  **发布目标**：修改 `gh release create` 指令，使其指向**公库**。
    *   例如：`gh release create v${{ env.VERSION }} -R wangn9900/StealthForward-Release ...`
3.  **Artifact 处理**：
    *   `web.zip`: 在私库生成，上传到公库 Release。
    *   `stealth-controller`: 在私库编译，上传到公库 Release。
    *   `install.sh`: **特殊处理**。脚本里的下载链接 (`REPO` 变量) 需要指向 **公库**。

### 第三阶段：授权系统隔离 (License Protection)
1.  **License Server**：
    *   建议将 `license-server` 目录彻底移出主项目，放入一个**独立的私有仓库**。
    *   因为它只有您自己用，不需要分发给用户，完全没必要即使是编译后的二进制也放在公库 Release 里（除非您打算卖授权系统本身）。
2.  **授权校验代码 (`internal/license`)**：
    *   这部分必须编译进 `stealth-controller`。
    *   **必须确保**：私库 Actions 在编译时，不仅是 Go Build，最好能加上 `go build -ldflags "-s -w"` (去除符号表) 甚至使用混淆工具 `garble`，防止逆向破解。

---

## 3. 特别注意事项

### 🚨 历史记录清洗 (Git History Cleanup)
如果现有的仓库转为私库，没问题。
但如果我们要创建一个新的公库，**绝对不能**直接 fork 或保留 `.git` 目录！
*   **操作**：在公库初始化时，必须是一个全新的 `git init`，只包含 `README.md` 和 `install.sh`。
*   **原因**：防止有人翻 GIT 历史记录找回即使现在已删除的源码。

### 🚨 `install.sh` 的修改
公库里的 `install.sh` 中的 `REPO` 变量必须修改为公库的地址 (e.g., `wangn9900/StealthForward-Release`)。
同时，Fallback 逻辑（源码下载）在公库模式下可能会失效（因为公库没源码）。
*   **解决方案**：修改 Fallback 逻辑，使其指向一个特定的 OSS 存储桶，或者完全移除 Fallback（只信任 Release）。

---

## 4. 任务清单 (Checklist)

- [ ] 创建/重命名私有仓库
- [ ] 创建全新的公有 Release 仓库
- [ ] 配置跨仓库发布的 GitHub Token
- [ ] 改造 `build.yml` workflow
- [ ] 改造 `install.sh` 以适配新仓库地址
- [ ] (可选) 增加 Go 代码混淆步骤
- [ ] 测试：在私库打 Tag，验证能否自动发布到公库
- [ ] **验证：确保公库中没有任何敏感源码泄露**
