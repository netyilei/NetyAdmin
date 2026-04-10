# 贡献指南

我们非常欢迎您为 NetyAdmin 项目做出贡献！您的帮助对我们来说至关重要。

在您开始贡献之前，请花几分钟阅读本指南。它将帮助您了解如何为项目做出贡献，并确保您的贡献能够顺利被接受。

## 行为准则

我们致力于为所有贡献者和用户提供一个开放和包容的环境。请阅读我们的 [行为准则](CODE_OF_CONDUCT.md)（如果存在），了解我们期望的行为规范。

## 如何贡献

### 报告 Bug

如果您在使用 NetyAdmin 时发现任何 Bug，请通过 [GitHub Issues](https://github.com/netyilei/NetyAdmin/issues) 提交。在提交 Bug 报告时，请尽量提供以下信息：

*   **重现步骤**：详细说明如何重现 Bug。
*   **预期行为**：您期望看到什么。
*   **实际行为**：实际发生了什么。
*   **环境信息**：您的操作系统、Go 版本、Node.js 版本、浏览器等。
*   **截图或视频**：如果有助于理解问题，请附上。

### 提交功能请求

如果您有新的功能想法或改进建议，也请通过 [GitHub Issues](https://github.com/netyilei/NetyAdmin/issues) 提交。请详细描述您的想法，包括：

*   **功能描述**：您希望实现什么功能。
*   **使用场景**：这个功能将如何帮助用户。
*   **替代方案**：您是否考虑过其他实现方式。

### 提交代码

我们非常欢迎代码贡献！请遵循以下步骤：

1.  **Fork (派生) 仓库**: 访问 `https://github.com/netyilei/NetyAdmin` 并点击右上角的 "Fork" 按钮，将仓库派生到您的 GitHub 账户。
2.  **Clone (克隆) 仓库**: 将您派生出来的仓库克隆到本地。
    ```bash
    git clone https://github.com/您的用户名/NetyAdmin.git
    cd NetyAdmin
    ```
3.  **创建新分支**: 为您的功能或 Bug 修复创建一个新的分支。请使用有意义的分支名称，例如 `feature/add-xxx` 或 `bugfix/fix-yyy`。
    ```bash
    git checkout -b feature/your-feature-name
    ```
4.  **进行更改**: 在新分支上进行代码修改。请确保您的代码符合项目的 [代码风格](#代码风格) 和 [提交信息规范](#提交信息规范)。
5.  **运行测试**: 如果项目包含测试，请确保所有测试通过。
6.  **提交更改**: 将您的更改提交到本地分支。
    ```bash
    git add .
    git commit -m "feat: add your feature" # 或 "fix: fix your bug"
    ```
7.  **推送到远程仓库**: 将您的本地分支推送到您派生出来的仓库。
    ```bash
    git push origin feature/your-feature-name
    ```
8.  **创建拉取请求 (Pull Request)**: 访问您派生出来的仓库页面，您会看到一个提示，引导您创建拉取请求。请确保将您的分支与 `netyilei/NetyAdmin` 的 `master` 分支进行比较。在拉取请求中，请详细描述您的更改内容、解决了什么问题或实现了什么功能。

### 代码风格

*   **Go**: 遵循 Go 官方的 [Effective Go](https://go.dev/doc/effective_go) 和 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) 规范。使用 `gofmt` 格式化代码。
*   **Vue/TypeScript**: 遵循 Vue 官方的 [风格指南](https://cn.vuejs.org/style-guide/)。使用 Prettier 和 ESLint 格式化和检查代码。

### 提交信息规范

我们建议您遵循 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/v1.0.0/) 规范来编写提交信息。这有助于我们更好地跟踪更改历史。

示例：

*   `feat: 添加用户管理功能`
*   `fix: 修复登录页面 Bug`
*   `docs: 更新部署文档`
*   `chore: 更新依赖`

## 代码审查

所有拉取请求都将经过代码审查。请耐心等待，我们会在审查过程中提供反馈。请准备好根据反馈进行修改。

## 感谢

感谢您对 NetyAdmin 项目的兴趣和贡献！
