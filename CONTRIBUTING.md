# 贡献指南

感谢你对 sath 的关注与贡献。请按以下方式参与：

## 开发环境

- Go 1.20+
- 设置 `OPENAI_API_KEY` 以运行对话/工具相关示例与测试

## 工作流

1. **Fork 本仓库**，在本地克隆你的 fork。
2. **创建分支**：`git checkout -b feature/xxx` 或 `fix/xxx`。
3. **修改与测试**：
   - 修改代码后运行 `go build ./...` 与 `go test ./...`。
   - 保持与现有风格一致（格式、命名、注释）。
4. **提交**：使用清晰的 commit message，可引用 Issue 编号（如 `fix #123`）。
5. **推送并提 PR**：推送到你的 fork，在本仓库打开 Pull Request，填写 PR 模板。

## 分支与 PR 约定

- 主分支：`main`，用于稳定发布。
- 功能/修复在单独分支完成，通过 PR 合并到 `main`。
- PR 需通过 CI（若有）及至少一次维护者 review 后再合并。
- 大改动建议先在 Issue 中讨论方案，再动手实现。

## 代码与文档

- 新增公开 API 请在 `docs/api-reference.md` 或对应文档中补充说明。
- 行为变更或新特性建议在 CHANGELOG 或 Release notes 中体现。

## 问题与建议

- **Bug**：请用 [Bug 报告模板](.github/ISSUE_TEMPLATE/bug_report.md) 提交 Issue。
- **功能建议**：请用 [功能建议模板](.github/ISSUE_TEMPLATE/feature_request.md) 提交 Issue。

再次感谢你的贡献。
