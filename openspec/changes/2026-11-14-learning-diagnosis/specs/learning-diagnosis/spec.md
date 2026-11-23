## Purpose

从教师确认的最新作答终评构建可追溯的个人和班级学习诊断，明确样本、分数、错因和知识点的统计口径，并为学生和教师提供受权限定的页面、模型解释与只读工具，避免初判和历史改分重复计数或越班读取。

## ADDED Requirements

### Requirement: D1 Confirmed error tags
系统 SHALL 兼容增加教师错因标签，合法标签为concept/calculation/reading/method/careless/other，各最多一次；最多6项，正确题标签为空，head及history同事务保存。

#### Scenario: Review compatibility and conflict
- **GIVEN** 原终评接口与旧记录
- **WHEN** 教师提交合法标签、改判、旧客户端不传标签或并发同版本保存
- **THEN** 合法标签可重开，正确题清空，未传错误题保留已有标签；并发200/409，历史不覆盖；非法/重复/正确题非空标签400且无写入，旧接口其他字段语义不变

### Requirement: D2 Deterministic authorized report
系统 SHALL 只统计最新作答的当前教师终评head，支持五学科、默认30天/1至180天的Asia/Shanghai自然日，显示总样本/学生数/得分率/正确率、知识点、错因频次、按提交日趋势和分页证据。

#### Scenario: Counting and correction
- **GIVEN** 已关联知识点的题目，含初判、教师终评及改分历史
- **WHEN** 查询、改判、重新作答、切换学科或日期
- **THEN** 初判及历史版本不计；同一题总样本一次，知识点各按关联计；无样本比例null，正式错误无标签计unclassified；改判即时更新，新作答未终评不沿用旧成绩；证据含当前终评版本，可追溯到具体作答

#### Scenario: Boundaries and authorization
- **GIVEN** 非法学科/天数/分页、身份字段注入、匿名、学生冒用student_id、他班/不存在学生或数据库故障
- **WHEN** 读取报告
- **THEN** 分别400/401/400/同形404/503；学生固定本人、教师限本班；超过10000样本400提示缩小范围；默认页1/20，上限100，稳定排序；100次串行普通查询在100条合成样本规模预热5次后p95不超过500ms

### Requirement: D3 Explanation of fixed statistics
系统 SHALL 让模型只解释已有汇总，不决定或改写分数、状态和统计；保留取消与失败重试。

#### Scenario: Grounded interpretation and failure
- **GIVEN** 已有正式样本或无样本
- **WHEN** 请求解释、输出非法数字/字段、取消、依赖失败或生成中教师改分
- **THEN** 有样本返回与报告指纹绑定的1至2000字解释；无样本不调用模型；不传学生身份/作答；非法输出503，指纹变化409，并发429；最多2次调用/121秒且取消不重试

### Requirement: D4 Read-only learning tools and page
系统 SHALL 提供诊断页面与已批准的知识点查询、作业统计MCP工具，调用现有业务服务并保持身份、班级与参数白名单。

#### Scenario: Native tool and page verification
- **GIVEN** 教师/学生页面及本地学习服务
- **WHEN** 查询页面、添加/连接/批准工具后调用或使用未批准/伪造参数的工具
- **THEN** 页面含加载/空/错误/重试与可回查样本；375/768/1280深浅主题无页面横向溢出；工具返回与业务接口一致的本班/本人数据，未批准或注入身份被拒，标准JSON-RPC的tools/call失败以isError/error判定，HTTP200不能冒充业务成功；直接工具调用每人每分钟最多30次，超限429/Retry-After=60；会话工具次数预算沿用现有流程，来源和工具返回不赋予权限
