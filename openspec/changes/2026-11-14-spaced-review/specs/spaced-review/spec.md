## Purpose
把当前有效错题与收藏知识点转为学生个人复习任务，保留可回查历史且不混入正式成绩。来源变化与重复操作均须保持一致。

## ADDED Requirements

### Requirement: R1 Source-bound personal cards
系统 SHALL 为当前有效错题与本人收藏提供幂等同步、查询与站内提醒。

#### Scenario: Sync and reconcile
- **GIVEN** 学生的错题、收藏及已有卡片
- **WHEN** 同步、重复同步、教师改判、重新作答、取消收藏或源删除
- **THEN** 同一源只有一张卡；当前失效源从待办排除，旧完成历史保留；他人/他班卡与不存在同形404，教师403，匿名401；新卡立即到期

### Requirement: R2 Deterministic idempotent completion
系统 SHALL 使用版本和请求键原子保存等级、下次到期与完成记录。

#### Scenario: Fixed schedule and retry
- **GIVEN** 固定服务端时间、等级0至5及默认1/3/7/14/30间隔
- **WHEN** 反馈good或again、重复或并发请求
- **THEN** good升一级至5并按新等级间隔，again归0取首间隔；到期为中国自然日加天数的09:00；同键同输入返回原回执，异输入或旧版本409；未到期或源失效409，不重复推进或写历史

#### Scenario: Settings and failure
- **GIVEN** 本人设置版本与既有到期日
- **WHEN** 保存五个严格递增的1至365整数、非法间隔、版本冲突或数据库故障
- **THEN** 合法设置只影响未来反馈；非法400、冲突409、故障503，旧到期及历史保留；取消或刷新不伪造完成

### Requirement: R3 Replayable page and tool
系统 SHALL 提供分学科复习页面、个人历史和只读复习计划工具。

#### Scenario: UI and authorized tool
- **GIVEN** 学生页面和经批准的review_plan工具
- **WHEN** 查看到期/全部、完成、历史、刷新、工具查询或错误恢复
- **THEN** 页面可回查源，显示等级与到期，含加载/空/失败/重试；375/768/1280深浅主题无横向溢出；工具与业务接口一致，仅本人最多5卡，不修改计划/正式成绩，身份和时间注入被拒；100次串行到期读取预热5次p95≤500ms
