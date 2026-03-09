---
name: python-data-helper
description: >
  使用 Python 脚本做轻量数据整理与格式化（如 JSON 校验、CSV 转表、简单统计）。
  适用于需要执行脚本完成数据清洗、格式转换或本地计算的场景。
tags: [python, data, script, format]
scope: [chat]
allowed_tools:
  - load_skill
  - read_skill_file
  - execute_skill_script
---
# Python 数据助手 Skill（python-data-helper）

## 何时使用

- 用户需要做 JSON 校验、格式化、CSV 解析或简单统计等，且希望通过执行脚本完成。
- 本 Skill 在 `scripts/` 下提供若干 Python 脚本，可在配置允许脚本执行时通过 `execute_skill_script` 调用。

## 工作流建议

1. **确认需求**：明确输入数据形式（JSON/CSV/纯文本）与期望输出。
2. **选择脚本**：根据 `scripts/` 下的说明选用合适脚本（如 `scripts/format_json.py`、`scripts/stats.py`）。
3. **执行**：调用 `execute_skill_script("python-data-helper", "scripts/xxx.py")`（需配置开启 `allow_script_execution` 且白名单包含 `.py`）。
4. **解读结果**：将脚本 stdout/stderr 结合上下文返回用户。

## 捆绑脚本说明

- **scripts/version.py**：输出 Python 版本与技能名，用于验证脚本执行（无需输入）。
- **scripts/format_json.py**：从 stdin 读取 JSON，格式化后输出（便于校验与阅读）。
- **scripts/stats.py**：从 stdin 读取多行数字，输出行数、和、均值等简单统计。

可通过 `read_skill_file("python-data-helper", "scripts/README.md")` 查看脚本用法（若存在）。

## 注意事项

- 脚本执行需在配置中开启 `skills.allow_script_execution`，且 `script_allowed_extensions` 包含 `.py`。
- 输入数据可通过管道或脚本内约定方式传入；当前脚本以 stdin 为例。
