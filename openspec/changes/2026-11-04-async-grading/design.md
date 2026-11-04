## Context

范围见proposal.md。现有平台具备认证、资料、检索、智能体与MCP；作业批改作为新增模块直接采用异步设计。运行整合沿用既有API，新增SQL只在grading的数据访问层。

## Goals / Non-Goals

**Goals:** 短请求受理、持久待办、独立消费、结果唯一、故障可恢复，所有判据能在隔离环境重放。

**Non-Goals:** 不引入主观题自动定分、跨校业务或消息端到端恰好一次保证。

## Decisions

### D1. 模块边界与契约

新增grading模块分为HTTP边界、业务服务、存储和网关适配，独立worker与relay命令。教师POST/GET /api/assignments，学生POST /api/assignments/{id}/submissions，授权读取GET /api/submissions/{id}与GET /api/tasks/{id}。列表采用默认20、上限100的稳定分页；客户端不能指定角色、班级、分数或状态。既有资料、技能、提示词与MCP接口保持兼容。

作业先支持客观题，得分由确定性规则计算，模型仅生成反馈。标准答案在受理前对学生隐藏；返回结果通过结构校验，错误只暴露分类码。教学反馈提示词有固定版本，执行日志记录actor_id、skill_id、status、duration_ms、token用量和task_id，不记录答案或原文。

### D2. 关系模型与事务发件箱

增加grade_assignments、submissions、grade_tasks、grade_results、grade_outbox及迁移版本表；不改已有占位表。资源保留class_id、created_by与UTC时间。关键约束：提交唯一键(student_id,assignment_id,client_key)、任务唯一submission_id、结果唯一idempotency_key，后者由grade:<submission_id>:v1派生。

受理事务写三处后提交；同键并发由唯一约束处理，再读取已提交任务并比较答案；不同答案409。relay短事务领取到期事件并设置租约，提交后在事务外发布持久消息并等待publisher confirm，最后更新已投递状态。崩溃可重投，因此不承诺物理消息只投一次；正常恢复场景检查逻辑任务与结果唯一。

### D3. RabbitMQ与执行租约

采用持久队列、持久消息、手工ACK、prefetch=1；死信通过独立可靠发件箱事件投到持久死信队列。选择RabbitMQ以复用确认与重投；不另造内存队列或数据库抢占消费者。

消费者短事务锁任务，领取随机run_token与租约、attempts加一，然后在事务外调网关。结果事务重新校验run_token与running状态，唯一写结果并置succeeded，提交后才ACK。完成消息重投只确认，不重复调用模型；旧消费者租约失效后不能覆盖。消费者异常终止时由代理重投，待租约过期接管。

### D4. 重试、失败与配置

重试由任务层统一承担，不叠加网关客户端重试。可重试失败在同一事务写pending与带available_at的重试事件，指数退避加抖动；永久失败和上限失败写failed与死信事件。数据库或代理故障保留持久待办，错误日志不含连接串。

新增RABBITMQ_USER/PASSWORD、AMQP_URL、GRADE_TIMEOUT_SECONDS、GRADE_MAX_ATTEMPTS、GRADE_LEASE_SECONDS、GRADE_RETRY_BASE_MS、OUTBOX_POLL_MS；仅键名样例，运行值外置。默认生产目标60秒/3次/70秒，隔离验证用更短预算并记录。作为异步执行的窄例外，模型请求不使用通用AI规则的内联重试，退避与上限由本任务统一管理。

### D5. 隔离复现与证据

Compose项目、端口、数据卷与现有运行环境分开，仅web绑定本机端口。自动回归使用合成A/B班及本地OpenAI兼容网关替身，真实数据库和消息代理照常运行；报告明确此范围。通过改变替身延迟、503/401序列、停止消费者/relay/broker和重复消息，观察任务表、结果与队列指标。

三条判据进apitest/cases并纳入make check；资产扫描独立make check-security。安全基线观察既有功能，红项保留，不借故放宽门禁。SEC-07分别核对已有教学技能与新增批改日志，不能用新日志掩盖已有缺口。恶意文档仅准备素材，交叉评审由成员实际复现。

## Risks / Trade-offs

- [代理确认与数据库更新间崩溃] → 接受至少一次投递，以结果唯一约束和租约校验吸收重复。
- [持久队列丢失或数据库恢复到旧版本] → 运维备份与恢复另验，单机测试不等同高可用保证。
- [既有依赖或审计字段有缺口] → 保存真实红项和处置计划，相关代码保留待评审状态。

## Migration Plan

先接入最小平台与队列，再应用可重复迁移、实现受理、投递、消费及查询，最后接页面与复现套件。回滚停止新入口、relay和消费者，保留新增表与待办；恢复前版应用不删除提交或结果，恢复新版本后继续消费。正式合入须经非作者评审。
