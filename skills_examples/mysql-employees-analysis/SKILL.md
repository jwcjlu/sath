---
name: mysql-employees-analysis
description: >
  将中文业务问题转换为 SQL 查询并分析 MySQL employees 示例数据库。
  适用于员工信息查询、薪资统计、部门分析、职位变动历史等场景。
version: 1.0.0
tags: [database, mysql, sql, employees, analysis]
scope: [dataquery]
allowed_tools: [list_tables, describe_table, execute_read]
---

## 概述

本 Skill 指导你基于 **MySQL employees 示例库**（常见表：employees、departments、dept_emp、dept_manager、salaries、titles）完成员工、部门、薪资与职位相关的分析任务。

## 前置条件

- 必须已配置 dataquery Agent，且存在可用的 MySQL 数据源并指向 employees 示例库。
- 本 Skill 仅使用只读工具：list_tables、describe_table、execute_read。

## 工作流程

1. **探索**：先调用 list_tables 确认当前库中的表（如 employees、salaries、dept_emp 等）。
2. **结构**：对关键表调用 describe_table，了解主键、外键与字段含义（如 emp_no、dept_no、from_date、to_date）。
3. **查询**：根据用户问题编写 SELECT，注意关联条件与时间范围（salaries、titles、dept_emp 等表常有 from_date/to_date）。
4. **解读**：用简洁中文总结查询结果，必要时区分「当前」与「历史」数据。

## 常见模式/模板

- **某部门当前员工**：dept_emp 关联 employees，过滤 to_date = '9999-01-01' 与 dept_no。
- **某员工薪资历史**：salaries 表按 emp_no，按 from_date 排序。
- **部门人数/薪资汇总**：dept_emp + salaries，按 dept_no 分组，注意 to_date 过滤当前有效记录。

## 最佳实践

- 涉及 salaries、titles、dept_emp 时，若不强调历史，优先过滤 to_date = '9999-01-01' 表示当前有效。
- 大表查询尽量带 LIMIT 或聚合，避免一次性返回过多行。
- 不要编造数据：工具返回为空时明确告知「未查到数据」。

## 示例

- 用户：「技术部有多少人？平均薪资多少？」
- 你：加载本 Skill 后，list_tables / describe_table 确认部门与薪资表结构，再 execute_read 执行关联查询与聚合，最后用中文总结人数与平均薪资。

## 故障排查

- **表不存在**：先 list_tables 确认库中实际表名（如 employees 库可能表名带前缀）。
- **关联错误**：检查 dept_emp、salaries 的 emp_no、from_date/to_date 含义，避免重复计算。
- **结果为空**：确认 dept_no、部门名称与 to_date 条件是否符合示例数据。
