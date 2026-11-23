# 安全审计实现复核（一）

日期：2026-11-16；基线 `c0d35f0004f60a3a6ec7cac6827dc15061e9ba1a` 加本地工作树。范围：`2026-11-16-security-audit` 的 S1–S4；依据 `harness/rules/03-security.md`、`04-testing.md`、`08-review.md`。这是代码自查与自动验证，不代表非作者批准。

| 维度 | 检查对象、判据与结果 |
| --- | --- |
| 需求覆盖率 | S1持久化/拒绝/故障，S2查询权限与分页，S3三类注入，S4门禁与模型实验均有对应实现及验证入口；成员独立复现仍需本人执行。 |
| 需求一致性 | 保持Bearer认证和旧业务接口，未将Cookie SameSite冒称已配置；教学提示词未改，实际模型现象单独记录。 |
| 失败路径 | 审计预留失败返回503且不进入业务；完成写失败保留started，不伪造完成；取消保留实际状态。 |
| 查询性能 | auditlog/query.go的身份、技能、时间均参数绑定并有班级复合索引；首轮105次含5次预热，p95=35.57ms。 |
| 重复代码 | 复用observability实际用量、auth身份与MCP执行层；未复制教学生成或评分逻辑。 |
| 安全风险 | auditlog仅接收身份、路由、对象、安全状态和数值；无原始输入/凭据。跨班详情与不存在同形404，学生查询403。 |
| 并发/事务/一致性 | 请求先预留再执行，批改尝试唯一键；复核发现审计故障可能消耗模型重试次数，已增加带run_token条件的释放；快照分页排除当前查询，防止审计自增造成重复。 |

## 修正与复审判据

- `grading/runner.go`、`worker_store.go`：最初预留在claim后失败会消耗一次尝试。修正为未调用模型时按run_token恢复pending和次数；以审计表暂不可达、网关调用数不变、恢复后成功验证。该路径及记录属于S1。
- `auditlog/query.go`、`securityaudit/http.go`：查询会产生新记录。固定snapshot_id/before_id并排除本次查询，连续两页不得重叠（S2）。
- `grading/gateway.go`、`models.go`：缺失usage与实际零用量须分开。通过缺省、null、真实零与非零响应验证Reported标识（S1）。
- `deploy/nginx.conf`：保留Host端口并覆盖转发协议，避免同源写请求误拒；用真实页面登录与接口反例验证（S3）。

## 文件结论

| 被审文件组 | 结论 |
| --- | --- |
| backend/internal/auditlog/{store,query}.go；runtimehttp/http.go；auth/auth.go | 可用，最终门禁与非作者评审另行确认。 |
| grading/{gateway,models,runner,worker_store}.go；cmd/{server,worker}/main.go | 已修正上述一致性问题，等待最终全量复审。 |
| observability/audit.go；curriculum/generation.go；mcp/runtime.go | 可用；只扩充安全元数据，不改变教学协议。 |
| securityaudit/{http,http_test}.go；auth/origin*.go；deploy/nginx.conf | 可用；权限、输入和来源边界有明确反例。 |
| frontend/src/pages/Audit.tsx；App.tsx；styles.css | 可用；三档布局、详情、错误与空态已在浏览器核对。 |
| MaterialTitle.tsx；Materials.tsx；frontend/tests/injection-render.test.mjs | 可用；检查实际组件编码，未用“没有弹窗”替代源码判据。 |
| scripts/check_security.py；apitest新增及扩展用例 | 可用；合成红反例不进入正常扫描或仓库，实际模型实验显式--live执行。 |
