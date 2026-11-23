# 安全审计实现复审

日期：2026-11-16；基线 `c0d35f0004f60a3a6ec7cac6827dc15061e9ba1a` 加本地工作树。规约：`2026-11-16-security-audit`，S1–S4；规则：03-security、04-testing、08-review。此记录为第二轮代码自查及自动验证，非作者批准和异机启动由成员另外确认。

| 维度 | 方法、对象、判据与结论 |
| --- | --- |
| 需求覆盖率 | 逐条反查S1–S4与tasks：持久审计、教师回查、三类注入、真实四步实验和交接均有入口；独立成员复现不以脚本代替。 |
| 需求一致性 | 对比本轮前基线与最终差异：未删除已有功能、未改教学提示词/终评逻辑/原安全规则，只有审计及请求来源边界增量。 |
| 失败路径 | 实测503前置拒绝、结束写失败保留started、取消/模型失败、缺失用量；检查{}、null、缺字段与真实零不能混淆，非法模型内容仍保留已上报用量。 |
| 查询性能 | 本班身份/技能/时间SQL带参数与复合索引，按id及固定上界分页；105次含5次预热，具体p95见audit-recheck，不超过500ms。 |
| 重复代码 | 复用身份、observability及MCP.Record；复核没有新增一套成绩逻辑或客户端授权。 |
| 安全风险 | 401/403/同形404、SQL固定输入、真实标题组件标记和跨站写入均检查；模型文本被注入的实际现象保留在sec08-experiment，不伪称完全消除。 |
| 并发/事务/一致性 | 唯一event_key限制同一尝试；预留失败按run_token恢复次数；取消后的结束记录有3秒独立预算；迁移重复与旧镜像回读20条样本不变。 |

## 第一轮问题的处理

- 实际用量：`grading/{gateway,models,usage_audit_test}.go`分别验证缺省、null、空对象、字段缺失、真实零及非零，并验证语义不合格的模型回复仍保留用量；不以默认0伪装实际上报。
- 未完成状态：`auditlog/store.go`与`runtimehttp/http.go`预留真实started，结束失败不改成成功；`securityaudit/http.go`排除当前查询并返回快照上界。故障注入和两页不重叠均有回归。
- 消费重试：`grading/{runner,worker_store}.go`按运行令牌释放未开始的模型调用；临时CHECK约束拒绝指定任务审计，网关调用数不变、attempts=0，恢复后成功。
- 验证环境：初始移表故障被启动迁移自愈，随后触发器受数据库权限限制；保留实际失败输出，最终使用CHECK约束，不改数据库安全设置、不删断言；两张表记录已保全合并。

## 文件结论

| 文件 | 结论与依据 |
| --- | --- |
| auditlog/{store,query,store_test}.go；runtimehttp/http.go；auth/auth.go；observability/audit.go | 可用；S1/S2权限、敏感元数据和异常出口已复核。 |
| grading/{gateway,models,runner,worker_store,usage_audit_test}.go；curriculum/generation.go；cmd/{server,worker}/main.go | 可用；实际/缺失用量、语义失败、唯一尝试及旧库兼容有验证。 |
| securityaudit/{http,http_test}.go；auth/{origin,origin_test}.go；mcp/runtime.go；deploy/nginx.conf | 可用；S2/S3服务端授权、分页白名单、跨站写入及工具拒绝均可重放。 |
| frontend/src/{App.tsx,styles.css,pages/Audit.tsx,pages/Materials.tsx,components/MaterialTitle.tsx} | 可用；教师入口、七列展示、详情返回、错误/空态与三档布局已检查，学生授权仍由接口决定。 |
| frontend/tests/injection-render.test.mjs；apitest/cases/api/{audit-events/GET.py,teaching-audit/POST.py}；apitest/cases/security/injections.py | 可用；使用真实持久化路径与实际标题组件，固定输入和故障均有断言。 |
| scripts/check_security.py；apitest/security_experiments.py；backend/internal/apidocs/static/openapi.json | 可用；真实扫描器红证明、完整正常扫描、真实模型实验和接口一致性分别验证。 |
| README、四件套与reports | 可用；检查结果按实际填写，规约不冒充人审，交接补丁仅包含本轮增量。 |

最终命令、退出码、源码指纹见security-fix-log；修复和重跑记录保留真实失败。全部结果仍需非作者按分工独立复现，不能以本复审代签。
