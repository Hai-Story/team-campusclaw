# 业务数据字典

批改迁移版本为001至005；API启动或`/app/grade-worker -migrate`在数据库迁移锁内执行；全部操作成功后才写版本，重复执行无数据变更。

业务时间使用UTC的DATETIME(6)；业务资源的class_id/created_by由服务端身份产生。任务状态、租约和尝试次数由服务端管理。终评分数仅由本班教师通过终评接口提交。

| 表 | 关键字段与含义 | 约束 |
| --- | --- | --- |
| grade_assignments | id、class_id、created_by、subject、title、question、expected_answer、max_score、created_at、updated_at | subject为chinese/math/english/physics/chemistry/biology/politics/history/geography；满分1至100；标准答案只对本班教师可读 |
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

## 知识点与收藏

迁移位于`backend/internal/curriculum/migrations/`，通过独立命名锁及`curriculum_migrations`只增版本执行。恢复旧版应用时保留新增表，原资料、作业、作答与成绩不改写。

| 表 | 内容与约束 |
| --- | --- |
| knowledge_points | 本班五学科知识点、章节、说明、创建人、UTC时间和版本；同班同学科名称唯一 |
| knowledge_point_materials | 知识点与本班资料关联，联合主键去重 |
| knowledge_point_assignments | 知识点与本班同学科单题作业关联，联合主键去重 |
| knowledge_favorites | 知识点与学生本人收藏，联合主键保证幂等 |

保存关联与版本更新在同一事务内校验；混入越权或异学科对象整笔拒绝。删除知识点仅清理其关联和收藏，不删除被关联的资源或成绩。

## 教师私有教案

`curriculum/migrations/002_lesson_plans.sql`只新增`lesson_plans`，不修改共享资料及作业表。

| 表 | 内容与约束 |
| --- | --- |
| lesson_plans | class_id、created_by共同限定本人草稿；subject、title、input_json、body、citations_json保存生成输入、正文与真实出处快照；client_key和input_hash用于幂等；version用于编辑和删除冲突校验；时间使用UTC |

同班同教师client_key唯一；同键不同输入返回409。来源删除或切片变化时标记不可回查并清空旧摘录，不删除教案正文。私有教案不写入共享资料、知识切片或向量索引；回退旧应用时保留新增表和草稿。

## 讲题过程

003迁移新增`walkthroughs`（本人班级、单题快照、阶段、版本、小结、建议及在途租约）和`walkthrough_turns`（本人回合、输入摘要、操作、前后阶段、输出、状态及执行令牌）。创建键按班级/本人唯一，回合键按过程唯一；参考答案只在服务端用于防泄露校验，不进入响应或模型输入。

阶段read→method→derive→check→completed由服务端按操作决定，失败/取消不推进；输出和阶段同事务保存，旧令牌不能回写。130秒租约过期后恢复失败，可在版本未变时同键重试；每过程最多50个回合键。只保存MySQL，不混入普通会话、正式作答或检索索引；回退保留新增表。

## 图谱与审核候选

004迁移新增`knowledge_graph_scopes`（班级/学科写锁）、`knowledge_graph_edges`（同学科端点、类型、版本、创建人）及`knowledge_graph_drafts`（教师私有输入、目录快照、候选节点/关系、出处快照、状态及版本）。正式节点复用knowledge_points；同学科最多500节点、2000关系，先修/包含不成环，关联关系按端点ID规范化。

审核时复核出处和原目录身份，同事务创建节点、资料关联与正式关系；失败全部回滚。pending/approved由服务端维护；同版本审核重试不重复创建。节点删除级联关系，已有关系时跨学科移动被拒。候选不进入共享检索；当前教师终评按每学生每作业最新作答计一次，reviewed>0且incorrect/reviewed≥0.5标为薄弱，学生只统计本人。

## 私有题库与试卷

005迁移新增`question_drafts`（生成输入、结构化题目、原始出处、审核状态/版本及入库ID）、`bank_questions`（本人题目、答案、解析、知识点、出处、题干摘要及版本）、`paper_drafts`（本人标题、有序题目快照、分值、总分、请求键及版本）。均以class_id和created_by共同隔离，时间使用UTC；不写入共享资料、切片或向量索引。

同教师同学科的题干去空白、转小写后计算摘要，并以唯一键阻止重复；保留数学运算符。草稿审核整批入库，任何失败全部回滚。题库改题须确认复核及当前版本；删除不影响已保存试卷。试卷1至50题，每题1至100分，总分最多1000；保存顺序、题面及题库版本，旧快照可继续排序/配分，明确选择新版本才替换。读取时复核出处有效性，同一响应合并重复来源检查，失效摘录不返回。

## 终评错因与诊断

批改004迁移为grade_reviews和grade_review_history增量添加可空error_tags JSON列。旧终评按空标签读取；错误终评可保存六种枚举标签，正确终评清空标签，当前头与历史版本同事务更新。迁移逐列检查，可重复启动；回退应用保留新增列。

诊断不另存成绩副本：仅取每学生每作业最新作答的当前终评，按中国自然日筛选；得分率为总得分/总满分，正确率为正确题数/样本数。同题关联多个知识点时，各知识点分别计样本，总样本不重复。指纹包含当前版本、关联与班级成员，生成解释前后复核。

## 间隔复习

006迁移新增review_settings（个人间隔及版本）、review_cards（学生/来源唯一，等级、到期与版本）和review_completions（请求键唯一、反馈、前后等级、原卡片与间隔快照）。设置与卡片更新按事务串行，源授权与有效性每次复核；同键同输入返回原回执，历史写入失败不推进卡片。时间存UTC，按中国自然日计算09:00到期；取消收藏或错题改判后历史保留，不改变正式成绩。

## 多题发布与提交

007迁移新增published_papers（班级发布、草稿版本唯一）、published_paper_questions（顺序、题目及出处快照、独立批改作业）、paper_attempts（学生整卷请求键及规范答案摘要）与paper_attempt_submissions（逐题作答映射）。发布和整卷受理均为单事务；同一学生行锁使并发整卷的逐题latest保持一致。答案/解析只在教师投影，成绩复用逐题终评当前头，全部终评前final_score为null。题库/草稿修改或删除不替换发布快照，回退时保留新增结构并启用只读入口。

## 九科迁移与示例批次

批改005与课程008将旧science归入physics、morality归入politics，同步教案/命题/图谱输入及复习完成快照中的subject；不改ID、题面、终评或关联。原幂等hash保留，过期输入可能409，重新发起需新请求键。回退使用迁移前数据库及uploads配套备份，不能将新增九科内容直接压回旧枚举。

009新增curriculum_seed_items，为显式导入的维护元数据，按(class_id,created_by,batch,unit_key)唯一；material_id/entry_id追踪真实讲义，status为staged或complete，resources_json记录创建的知识点、关系、命题草稿、题库及教案ID；created_at记录批次时间。它不提供业务读写接口，状态仅由维护命令推进；删除本批资料时台账随外键清理。讲义先保存并索引，私有资源按主题事务提交，完成主题不自动补建或覆盖编辑。

显式作业导入在同一resources_json增记homework_single_id、homework_paper_id。单题及其标记同事务；多题草稿使用junior-homework-v1-<subject>请求键并复用发布幂等。标记存在即跳过，不补建已删除作业、不生成学生提交或成绩；失败可从已有草稿续办。

## 作业提醒

010新增student_homework_notifications，保存班级、发布教师、接收学生、单题或整卷ID、创建时间及首次已读时间；两个资源ID恰有一个非空，学生/资源唯一。新发布与当前同班学生提醒同事务提交，整卷子题不单独提醒；已发布作业不补发。查询按当前身份和班级隔离，已读幂等，作业删除级联清理。回退应用保留新增表及记录，重新升级后继续读取。
