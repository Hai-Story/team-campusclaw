## Why

教师需要从已确认作答中识别薄弱知识点，并回查统计依据；AI初判与历史改分不能冒充正式成绩。

## What Changes

- 终评增加可选错因标签，保留版本与历史。
- 两端按学科和日期查看正式样本、得分率、正确率、知识点及趋势；教师查看本班或指定学生。
- 模型解释确定性统计，过期解释拒绝；知识点和正式作业统计接入只读本地MCP工具。

## Capabilities

### New Capabilities
- `learning-diagnosis`: 教师终评驱动的诊断与只读业务工具。

### Modified Capabilities

无。

## Impact

grading终评兼容字段及迁移、curriculum统计、两端页面、MCP注册执行与OpenAPI。无新依赖。Non-goals：互评、排名、自动给正式分数、复习调度与多题发布。
