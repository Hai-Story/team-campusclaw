## Context
现有资产检查报告npm和Go风险，SEC-07普通会话缺少actor_id/skill_id/usage；可选私有trace不能替代普通审计。

## Goals / Non-Goals
使依赖检查与SEC-07实际通过，保留原会话、工具和权限语义。既有评审与安全规则原文保持不变。

## Decisions
- 采用受支持且已修补的Go稳定补丁版及Vite维护分支，锁定镜像、模块与npm锁文件；构建、扫描使用同一Go版本。升级后以真实扫描结果判定，不降低风险阈值。
- 增加请求级、只收集数字与标识的审计上下文，和详细trace分别管理。每次模型HTTP请求记录尝试/成功/失败/取消及模型名；流式请求显式申请include_usage，从非流式JSON或流式usage收集真实token数，累加全部规划与回答请求。缺失用量明确标记，不伪造为已报告0。
- 教学会话绑定服务端actor_id及选中的skill_id（无技能为空），结束时产生同request_id的JSON记录，含module/event/status/duration_ms/model/usage与调用次数。任何早退或取消仍记录；无调用时calls=0且reported=false。日志不包含会话正文、工具参数、模型输出、token或密钥。
- 私有trace仍须显式开启、仅本人读取；普通审计不创建trace正文副本。会话持久化、SSE和工具预算保持兼容。

## Validation
正常、模型失败、断流/取消、无技能及多轮工具检查真实累计用量和状态；合成正文与口令标记不得出现在日志。SEC-01至07及全部接口/Go/前端回归，依赖扫描无已知发现，密钥扫描0命中。规则不改、失败保持非零。
