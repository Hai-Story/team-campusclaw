## Purpose

让学生围绕本班单题作业逐步表达审题、方法、推导和检查过程，保存教师可解释的学习记录与改进建议。过程独立于普通对话和正式成绩，通过本人授权、确定阶段及可恢复的生成状态支持连续练习。

## ADDED Requirements

### Requirement: W1 本人讲题记录

系统 SHALL 提供学生POST/GET /api/walkthroughs和GET /api/walkthroughs/{id}。创建输入assignment_id为本班正整数、client_key为1至64位ASCII字母数字或连字符；同键同作业返回同记录，同键异作业409。列表按五学科subject（默认math）、page=1、page_size=20（最大100）及q（0至100字）过滤。响应包含题面、阶段、version和按ID正序的回合，不能包含参考答案；仅本人本班可读，教师403，其他学生与不存在同形404。参数非法400、未登录401、依赖故障503。

#### Scenario: W1-S1 创建与隔离
- **GIVEN** 同班两名学生及他班学生
- **WHEN** 一人重复创建并由其他人读取
- **THEN** 仅一份本人记录，其他人同形404；学生原作业与成绩不变

### Requirement: W2 阶段与幂等回合

系统 MUST 提供POST /api/walkthroughs/{id}/turns，输入content为trim后1至1000字、action为discuss/advance/finish、expected_version为正整数及client_key。阶段依次read/method/derive/check/completed；discuss保持阶段，advance要求当前阶段至少一条成功discuss且不能用于check，finish只在check存在成功discuss时完成。成功version加一；每记录最多50个回合键。同键同输入成功结果可重放且不再次调用模型；同键不同输入或旧版本409；同记录生成中409。失败或取消不推进阶段，可在版本未变时同键重试。

#### Scenario: W2-S1 完整过程与冲突
- **GIVEN** 新讲题记录
- **WHEN** 各阶段讨论后依次推进并完成，再重放请求或并发使用旧版本
- **THEN** 顺序确定，过程/总结/建议可恢复，重放不重复写入，冲突409，不产生分数

### Requirement: W3 SSE和故障恢复

系统 SHALL 复用解题引导，模型单次60秒、网络/429/5xx最多重试一次并退避1秒，总预算121秒；130秒在途租约过期恢复失败。输出response为1至1500字、summary为1至1000字、improvements为1至5项各1至600字；拒绝额外字段、答案泄露和虚构编号引用。SSE先meta，成功校验保存后delta和done；生成失败发error并保留失败回合，取消停止模型且不重试，不将部分结果当成功。取消发生在提交后时，刷新须返回已保存的成功记录。普通会话契约不变。

#### Scenario: W3-S1 取消故障与刷新
- **GIVEN** 在途生成或异常模型/数据库
- **WHEN** 取消、断流、刷新、进程重启后租约超时或返回畸形输出
- **THEN** 阶段不被失败推进，已提交记录仍可读取，未提交输出不可冒充成功；可恢复后重试，生成及成绩无重复副作用

### Requirement: W4 独立学生页面

学生 SHALL 有讲题模拟入口，按五学科选择单题、打开本人历史、讨论、取消、推进及查看小结；错误保留输入，刷新恢复选中记录。页面具备加载/空/错误状态，375/768/1280宽度及深浅主题可操作；教师无入口，普通对话继续可用。

#### Scenario: W4-S1 页面主路径
- **GIVEN** 已有本班作业和本人过程
- **WHEN** 创建、追问、推进、刷新及停止请求
- **THEN** 显示真实保存阶段和历史，能恢复与重试，无横向溢出
