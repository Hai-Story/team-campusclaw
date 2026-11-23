## Purpose
在原单题流程上增加不可变多题作业与逐题批改，并让学习扩展继续使用统一作答与教师终评。保留独立作业的既有接口与历史数据。

## ADDED Requirements

### Requirement: M1 Immutable publication
系统 SHALL 将教师本人私有试卷指定版本原子发布为本班同学科作业，保留有序题面、分值与服务端参考快照。

#### Scenario: Publish and preserve
- **GIVEN** 合法试卷草稿、当前知识点和有效出处
- **WHEN** 发布、重复发布、并发发布、版本过期或题库随后改题/删除
- **THEN** 每版本只有一份发布；首次版本过期409，越权404，来源失效409；失败全部回滚；已发布题面和既有成绩不变；学生详情不含答案、解析或出处摘录

### Requirement: M2 Atomic whole-paper attempts
系统 SHALL 对一卷1至50题整份受理，使用请求键去重并复用逐题任务与发件箱。

#### Scenario: Submit and retry
- **GIVEN** 本班学生及发布题目
- **WHEN** 按题型提交全部答案、重复或并发同键提交
- **THEN** 202含同一组作答与任务标识；同键同规范答案复用，异答案409；缺题/重复/越界/非法题型输入400，超1MiB为413，事务中途失败503且无孤立写入；学生无法读取他人作答，越班与不存在同形404

### Requirement: M3 Per-question final review and total
系统 SHALL 让AI仅初判，每题由教师独立确认最终对错、分数和错因，总分使用当前终评计算。

#### Scenario: Review and reconcile
- **GIVEN** 多题作答、部分初判和教师终评
- **WHEN** 终评、改判或提交新一轮整卷
- **THEN** 正式总分在全部终评前为null；之后等于各题终评分和，总满分等于发布分值和；错题按当前有效判断同步，诊断/图谱只计最新作答当前终评，历史不重复；复习移出改判正确的待办，保留完成记录

### Requirement: M4 Legacy and page compatibility
系统 SHALL 保留独立单题行为，提供可回查的多题交互和扩展跳转。

#### Scenario: Existing and new paths
- **GIVEN** 原单题与已发布多题
- **WHEN** 查询/作答/终评、刷新、跳转或依赖故障
- **THEN** 旧单题接口与记录仍可用，原列表不混入父卷题；父卷题经旧单题提交409；两端可按学科选择模式、逐题操作并恢复已提交记录，知识点/错题/图谱/诊断/复习和讲题可回查对应题；页面具备加载/空/失败/重试，375/768/1280深浅主题无横向溢出；100次列表预热5次p95≤500ms
