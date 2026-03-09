# 脚本说明

- **version.py**：输出 Python 版本与技能名，无需输入，用于验证执行。
- **format_json.py**：从 stdin 读 JSON，格式化输出。示例：`echo '{"a":1}' | python3 scripts/format_json.py`
- **stats.py**：从 stdin 读每行一个数字，输出行数、和、均值。示例：`echo -e "1\n2\n3" | python3 scripts/stats.py`
