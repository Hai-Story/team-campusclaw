## Purpose

为学生当前有效错题提供基于本班真实资料的学习引导，保证检索、模型输出和出处都受现有身份及答案边界约束；资料或教师判断变化后不展示过时讲解，同时保留原有按钮与接口兼容性。

兼容：本能力扩展A13的讲解成功响应与时限，保留explanation字段及既有初判、终评和错题归属语义。

## ADDED Requirements

### Requirement: E1 Authorized evidence
系统 SHALL 仅允许学生本人当前有效错题生成讲解，检索最多4条本班同学科或其他资料，来源须含真实资料/切片标识、范围、标题与安全摘录。

#### Scenario: Cited guidance
- **GIVEN** 当前错题及本班可检索资料
- **WHEN** 请求原explanation接口
- **THEN** 返回explanation、status=generated及被引用的真实citations，题目答案不出现在讲解和出处标题/摘录中；不改变作答

#### Scenario: Missing evidence
- **GIVEN** 检索没有授权命中
- **WHEN** 生成讲解
- **THEN** 200、status=insufficient_evidence、空出处与明确提示，模型调用数0

#### Scenario: Invalid and unauthorized
- **GIVEN** 非法标识或额外正文、未登录、教师、他人/他班或不存在资源
- **WHEN** 请求讲解
- **THEN** 分别400/401/403/同形404且不调用模型

### Requirement: E2 Validated output and current state
系统 SHALL 仅接受1至1500字讲解及非重复真实引用编号，拒绝答案泄露、伪造引用、额外字段；生成结果不能覆盖新错题状态。

#### Scenario: Stale or malformed result
- **GIVEN** 在途生成
- **WHEN** 资料删除/修改，教师终评版本变化或学生重新提交
- **THEN** 409且无成功正文；畸形/泄露/伪造输出503

#### Scenario: Failure and cancellation
- **GIVEN** 依赖失败或在途生成
- **WHEN** 检索不可达、模型失败、同学生并发或取消
- **THEN** 503、受控最多2次模型调用及121秒预算、并发429；取消终止且不重试，页面保留重试入口

### Requirement: E3 Compatible page
系统 SHALL 保留原按钮、加载、取消与重试，并显示可授权回查的出处。

#### Scenario: Existing interaction
- **GIVEN** 学生错题页面
- **WHEN** 生成/取消/失败重试及切换错题
- **THEN** 文本与出处属于当前错题，旧请求不串入；375/768/1280深浅主题无页面横向溢出
