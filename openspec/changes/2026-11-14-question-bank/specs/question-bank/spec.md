## Purpose

为教师提供从知识点与讲义出发的命题、修改、审核和题库管理流程，并通过私有试卷草稿保存选题顺序、题面和配分。未审核内容、答案和解析不对学生开放，来源可核对，内容生产不改动既有作业与成绩。

## ADDED Requirements

### Requirement: B1 私有生成与严格题目结构

所有JSON请求正文总限128KiB，超过返回413；单字段上限仍分别校验。

系统 SHALL 提供教师POST /api/question-drafts/generate、GET /api/question-drafts和GET/PUT /api/question-drafts/{id}。生成包含subject五学科、knowledge_point_ids为1至10个本班同学科正整数、question_types为single_choice/multiple_choice/true_false/short_answer中1至4个不重复允许类型、difficulty为easy/medium/hard、question_count整数1至10、material_ids为1至10个本班资料及client_key（1至64位ASCII字母数字或连字符）。仅检索指定资料最多6条命中；无命中200、status=insufficient_evidence、draft=null且不调用生成模型。同键同输入复用草稿，异输入409，在途429，模型预算单次60秒、网络/429/5xx最多重试一次并退避1秒、总121秒，代理读取等待至少130秒以返回受控超时错误；取消不保存。

每题包含type、stem(1至2000字)、options、answer、explanation(1至2000字)、knowledge_point_ids、difficulty、citation_indices。单选2至8个不重复选项且1个A至H范围答案；多选2至8选项且2至选项数个不重复范围答案；判断无选项、一个true/false答案；简答无选项、一个1至2000字参考答案。每题知识点1至10个且在授权输入范围，引用至少一个实际命中且不重复；难度及题型须符合请求，初始题数必须一致。整批重复题或非法模型输出503且无草稿；已有题库重复题409。PUT携带expected_version，仅pending可改，允许教师删减为1至10题；逐题错误说明题号及原因。

#### Scenario: B1-S1 生成修改与隔离
- **GIVEN** 本人知识点、本班讲义与另一教师/学生
- **WHEN** 生成、重试、编辑及跨用户读取草稿
- **THEN** 结构/题量/出处符合请求、同键不重复生成；本人保存，另一教师同形404、学生403，答案不进入共享检索

#### Scenario: B1-S2 失败边界
- **GIVEN** 非法选项/答案/重复题、无依据、他班知识点/资料或不可用依赖
- **WHEN** 生成或编辑
- **THEN** 非法输入400、越权同形404、无依据明确提示、依赖或坏输出503、重复已有题409，均无部分草稿或题库写入

### Requirement: B2 教师审核与私有题库

系统 MUST 提供POST /api/question-drafts/{id}/approve及GET /api/bank-questions、GET/PUT/DELETE /api/bank-questions/{id}。审核携带expected_version，重新校验知识点、出处及重复；同事务创建1至10条题库记录并标草稿approved，来源变化409且全部回滚；已审核同版本重试复用结果，不重复建题。PUT题库包含question、expected_version与confirm_review=true，教师显式复核后版本加一；新知识点仍须同班同学科，重复题409。删除需当前版本，仅删除本条；已保存试卷快照保留。题库读取包含本人答案/解析/实际出处，失效出处标记并清空摘录。所有对象按创建教师与班级共同过滤，学生403、其他教师与不存在同形404；未登录401、非法400、依赖故障503。列表subject默认math，q长度0至100、page=1、page_size=20（最大100），按ID倒序。

#### Scenario: B2-S1 审核与并发重复
- **GIVEN** 本人待审题目及有效出处
- **WHEN** 审核、重试、并发同版编辑或审核重复题，并尝试他人读取
- **THEN** 一份有效题库结果，冲突409且无半批题目；隐私403/404，编辑须确认复核

#### Scenario: B2-S2 来源变动与删除
- **GIVEN** 审核前来源变动或已入库题目被选进试卷
- **WHEN** 审核失败、编辑题目或删除题库条目
- **THEN** 失败不改变审核状态，已选题面与原分数快照保留，失效来源不冒充可回查

### Requirement: B3 有序试卷草稿

系统 SHALL 提供教师POST/GET /api/paper-drafts和GET/PUT /api/paper-drafts/{id}。创建subject、title为trim后1至120字、items为1至50项有序且question_id不重复，每项question_id为正整数、question_version为正整数、score为1至100整数，总分至多1000；创建还含client_key。新选题必须属于本人本班同学科且版本匹配，否则404或409。保存每题完整快照与顺序，服务器计算总分；题库后续改动不更改快照。修改携带expected_version，既有相同题目/版本沿用服务器旧快照，可继续排序配分；明确传入题库新版本时再读取新题面。创建同键同输入返回同草稿，异输入409；更新旧版409。权限、分页、失败状态同B2；本轮无发布接口，学生不能访问试卷内容或答案。

#### Scenario: B3-S1 组卷与快照
- **GIVEN** 本人多条已审核题目
- **WHEN** 选题、排序、配分、保存重开，再修改/删除题库原题并保存试卷顺序
- **THEN** 总分等于逐题分值之和，顺序一致，原快照可恢复；未经明确更新不替换题面，非法/重复/越权项整笔拒绝

### Requirement: B4 教师页面与兼容

教师 SHALL 有智能命题与题库入口，按五学科配置知识点、题型、难度、题量与讲义，生成后逐题核对/修改/审核入库；支持题库编辑与确认复核、选题上下排序、配分及试卷保存重开。错误保留输入，生成可取消和重试；加载/空/无依据状态真实，375/768/1280与深浅主题可操作。学生无入口及接口权限，原单题、初判、终评、错题、备课、讲题与图谱不受影响。

#### Scenario: B4-S1 页面闭环
- **GIVEN** 教师和学生账号
- **WHEN** 生成、编辑审核、题库改题、组卷、刷新重开及取消失败请求
- **THEN** 教师流程可完成且失败可恢复，学生不能读取答案，旧功能回归通过
