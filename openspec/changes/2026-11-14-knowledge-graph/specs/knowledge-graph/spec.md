## Purpose

在本班知识点目录上展示先修、包含与关联关系，让教师维护经过确认的教学结构，学生点选知识点查找讲义和练习。模型提取先形成私有候选，审核后才进入正式图谱；薄弱节点依据教师终评且保留样本口径。

## ADDED Requirements

### Requirement: G1 正式图谱与维护

系统 SHALL 提供GET /api/knowledge-graph?subject（默认math，既有五学科）以及教师POST/DELETE /api/knowledge-graph/edges及/{id}。POST包含from_id、to_id、kind(prerequisite/contains/related)，两个不同节点须同班同学科，否则400或同形404；相同关系幂等返回原记录。先修及包含不得成环，关联无向去重；自环400、成环409且无写入。删除须expected_version正整数，旧版409。最多500节点和2000关系，超限409。节点已有正式关系时改学科409；删除节点只清理所属关系。未登录401、学生写403、非法输入400、数据库故障503。

#### Scenario: G1-S1 手动关系与并发成环
- **GIVEN** 同学科A/B/C节点及他班节点
- **WHEN** 建A→B、B→C后加C→A，或并发加A→B/B→A，或混入他班节点
- **THEN** 正式图无环、后到冲突409，越权同形404；重复关系不重复写入

### Requirement: G2 私有提取候选与审核

系统 MUST 提供教师POST /api/knowledge-graph/drafts/generate、GET /api/knowledge-graph/drafts和GET/PUT /api/knowledge-graph/drafts/{id}、POST /api/knowledge-graph/drafts/{id}/approve。生成subject、1至10个本班material_ids和client_key；仅取指定资料最多6条真实命中，无命中200、status=insufficient_evidence、draft=null且无模型调用。输出nodes为1至10项（name1至120字、chapter0至120、description0至2000），edges为0至20项（from/to引用候选或现有节点名称、kind同G1），引用编号必须来自命中，节点与边不得重复。非法模型输出503；生成预算沿用单次60秒/最多重试一次/退避1秒/总121秒，取消不保存。

候选仅创建教师本班可见（其他教师404、学生403）；列表按subject/q/page/page_size既有默认分页。生成同键同输入复用，异输入409，在途429。PUT携带expected_version并只编辑pending候选；审核要求同版本及有效来源，审核中引用失效、节点名称失效或成环409，整批无部分写入。审核复用同学科同名目录节点，不覆盖其说明；新增节点关联引用资料，关联总数不得超过50；已审核同版本重试返回原结果，不重复建点/边。

#### Scenario: G2-S1 提取编辑审核与隐私
- **GIVEN** 有效讲义与本人候选
- **WHEN** 提取、编辑、审核、重试以及学生/另一教师读取
- **THEN** 审核前正式图不含新增内容，审核后节点/关系/资料关联一致且幂等；私有内容不进入学生检索，越权403/404

#### Scenario: G2-S2 来源变化与原子失败
- **GIVEN** 待审候选
- **WHEN** 审核前资料删除或切片改变、构成先修环、并发使用旧版本或依赖故障
- **THEN** 分别409/409/409/503，无部分正式节点、关系或审核状态；可修正后重试

### Requirement: G3 资源与薄弱口径

系统 SHALL 提供GET /api/knowledge-graph/nodes/{id}/resources，限本班。返回本节点资料、同学科作业、直接先修节点及复习资料；只向学生本人返回有效错题，教师无学生错题正文投影。薄弱统计reviewed/incorrect按每学生每作业最新作答的当前教师终评计算，历史版本不重复；学生统计本人、教师统计本班，reviewed>0且incorrect/reviewed>=0.5标weak。AI初判不进入该统计。节点删除不删除原资料、作业或成绩。

#### Scenario: G3-S1 终评改判与资源
- **GIVEN** AI判错但未终评的作答，以及与资料/作业关联的节点
- **WHEN** 读取、教师终评判错、改为正确以及学生点选复习资源
- **THEN** reviewed从0变1，incorrect从0变1再变0，历史版本不增加样本；即时错题独立更新，本班资源可回查，他班节点同形404

### Requirement: G4 图谱页面

两端 SHALL 有五学科图谱入口，可点选及键盘选择节点、筛选关系类型并查看资源和薄弱样本；教师另有新增/删除关系、提取、候选编辑和确认审核入口。加载/空/错误/无依据状态真实，失败保留输入；375/768/1280及深浅主题无页面横向溢出，未审核候选不在学生正式图中出现。

#### Scenario: G4-S1 页面闭环
- **GIVEN** 有节点及待审候选
- **WHEN** 维护、点选资源、提取、修改及审核后刷新
- **THEN** 正式图与资源可恢复，角色入口正确，错误可重试
