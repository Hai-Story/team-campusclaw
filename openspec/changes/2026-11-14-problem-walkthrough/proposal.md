## Why

学生需要围绕一道作业持续解释自己的思路，并在中断后继续练习。独立保存讲题过程，避免与普通对话及正式作答混在一起。

## What Changes

- 新增学生本人的讲题记录，保存审题、选方法、推导、检查及完成阶段。
- 复用解题引导技能与SSE传输，保存追问、过程总结及改进建议；阶段由服务端按操作推进。
- 保留普通会话、单题提交、AI初判与教师终评。

## Capabilities

### New Capabilities
- `problem-walkthrough`: 本人作业讲题、阶段推进、幂等请求与中断恢复。

### Modified Capabilities

无。

## Impact

curriculum增量表、独立接口与学生页面，沿用现有模型适配及SSE解析器，无新增依赖。Non-goals：语音、互评、成绩写入、多题作业及错题检索改造。
