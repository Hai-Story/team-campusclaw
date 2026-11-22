## Purpose
通过同一组授权合成数据连接六项学习扩展、多题作业与旧单题链路，验证页面、接口、持久化及部署一致性，保留可重放证据并保留必要的运行说明。

## ADDED Requirements

### Requirement: I1 Connected learning workflow
系统 SHALL 在同班同学科完成教师内容准备、学生练习、教师终评和后续学习闭环。

#### Scenario: Complete and replay
- **GIVEN** 独立合成教师/学生与可检索讲义
- **WHEN** 备课、命题审核、组卷发布、整卷作答、讲题、终评、诊断、图谱和复习
- **THEN** 数据可回查、正式统计仅来自终评、重复请求不重复写入；非法400、角色403、跨班与不存在同形404、依赖503均保留；清理不影响已有记录

### Requirement: I2 Verified deployment
项目 SHALL 保留真实检查结果及可恢复的本地部署。

#### Scenario: Build deploy and preserve
- **GIVEN** 完整本地成果、运行备份
- **WHEN** 执行构建/回归、更新18080
- **THEN** Go/前端/接口/OpenAPI/规约检查结果可核对，旧数据保留、运行入口正常，镜像及报告不含凭据；真实模型未完成项及非作者验收保持待完成，不宣称发布通过
