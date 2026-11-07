# 作业批改数据字典

迁移：`backend/internal/grading/migrations/001_async_grading.sql`，版本`001_async_grading`、`002_teacher_review`、`003_unified_subjects`。API启动或`/app/grade-worker -migrate`在数据库迁移锁内执行；所有DDL成功后才写版本，重复执行无数据变更。

业务时间使用UTC的DATETIME(6)；业务资源的class_id/created_by由服务端身份产生。任务状态、租约和尝试次数由服务端管理。终评分数仅由本班教师通过终评接口提交。

| 表 | 关键字段与含义 | 约束 |
| --- | --- | --- |
| grade_assignments | id、class_id、created_by、subject、title、question、expected_answer、max_score、created_at、updated_at | subject为chinese/math/english/science/morality；满分1至100；标准答案只对本班教师可读 |
| submissions | id、assignment_id、student_id、class_id、created_by、client_key、answer、answer_hash、created_at、updated_at | (student_id,assignment_id,client_key)唯一；同键不同答案409 |
| grade_tasks | task_id（随机32位十六进制）、submission_id、class_id、created_by、status、attempts、idempotency_key、error、run_token、lease_until、available_at、started_at、finished_at、created_at、updated_at | submission_id与幂等键各自唯一；状态pending/running/succeeded/failed；仅当前run_token可提交 |
| grade_results | id、submission_id、class_id、created_by、idempotency_key、ai_correct、score、max_score、feedback、prompt_tokens、completion_tokens、created_at、updated_at | grade:&lt;submission_id&gt;:v1唯一；新初判score为NULL，旧数值保留；AI只写布尔判断与反馈；与任务成功同事务提交 |
| grade_outbox | id、task_id、kind、generation、state、claim_token、lease_until、publish_attempts、available_at、published_at、created_at | (task_id,kind,generation)唯一；kind为work/dead，state为pending/publishing/published；租约用于并发领取 |
| grade_reviews | id、submission_id、class_id、created_by（终评教师）、score、max_score、correct、feedback、version、created_at、updated_at | submission_id唯一；分数0至满分，对错独立确认；版本冲突409 |
| grade_review_history | 与当前终评相同的业务字段，保存每次版本 | (submission_id,version)唯一，只追加；与当前终评同事务 |
| grade_migrations | version、applied_at | 运维元数据，版本主键；不作为班级业务资源 |

outbox是从任务派生的投递元数据，归属通过task_id关联，不重复存储班级与操作人。状态仅由服务端推进；失败分类码不含模型原文、地址或凭据。成绩只保留这一提交版本的一份生效结果。

受理事务：submissions→grade_tasks→grade_outbox，任一步失败全部回滚。结果事务：校验租约→grade_results→任务成功，之后才ACK。外键级联只用于所属提交清理。教师更新终评时递增版本并追加历史。

回退保留新增表和数据，停止新增受理与消费者后恢复前版应用；恢复此版本继续消费。迁移重复执行、受理回滚与恢复通过隔离用例验证；生产备份恢复目标仍需单独演练。

旧作业由002迁移归入math，旧成绩不冒充AI判断。迁移前后既有3份结果内容/数值保持一致且重复迁移通过。有效错题按每作业最新提交与COALESCE(教师correct,AI correct)=false查询；未知判断不进入错题。

003将既有physics/chemistry作业归入science，关联作答/终评/历史不改写。作业与错题按五个具体学科筛选。班级资料保留五学科、其他和本班总览。
