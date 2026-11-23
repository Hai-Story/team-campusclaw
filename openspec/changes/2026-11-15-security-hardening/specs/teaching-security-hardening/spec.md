## Purpose
通过升级修补依赖和记录请求级教学审计，确保每次模型调用的主体、技能、结果与真实用量可追溯，同时保持既有权限、会话行为及敏感内容边界，不以放宽检查替代修复。

## ADDED Requirements

### Requirement: H1 Reproducible patched dependencies
项目 SHALL 固定修补后的构建链与依赖，并保留真实安全及功能检查。

#### Scenario: Upgrade and audit
- **GIVEN** 受影响的Go与npm依赖
- **WHEN** 升级、干净构建、执行现有扫描和回归
- **THEN** npm已知漏洞与Go可达通告均为0，历史/工作树/前端/日志密钥命中0；扫描失败或发现风险非零退出，功能测试保持通过

### Requirement: H2 Metadata-only teaching audit
系统 SHALL 为普通教学请求记录服务端主体、技能、模型、调用状态与真实累计用量，独立于可选私有trace。

#### Scenario: Calls failure and cancellation
- **GIVEN** 有/无技能、普通/多轮工具的授权教学会话
- **WHEN** 请求成功、模型失败、取消、断流或在调用前失败
- **THEN** 同request_id审计含actor_id/skill_id/status/duration_ms/model/usage；规划与回答用量按实际usage累计，缺失明确标记，取消不记成功；正文、答案、工具参数及凭据不进日志，关闭trace时不保存详细trace；原角色与跨班401/403/404及依赖503契约保留
