# 安全自查

> 来源记录：以下 2026-11-05 内容随 `codex/homework-teacher-review@df755cc` 交接导入，保留原受测提交与执行范围，不代表张海峰本次独立复测。2026-11-07 的集成与独立复测结果见 [review-01.md 的追加记录](review-01.md#2026-11-07-张海峰集成与独立复测)。

日期：2026-11-05。受测HEAD：`ba61ce5f639fa45f40e566be171e26b67bc42847`，含本次回归与记录更改；隔离合成环境。

## 接口范围

未登录应返回401：`GET /api/me`、`GET /api/materials`、`GET /api/agents`、`GET /api/skills`、`POST /api/search`、`POST /api/assignments/{id}/submissions`、`GET /api/tasks/{id}`、`GET /api/submissions/{id}`、`PUT /api/submissions/{id}/review`。

写入检查：`PUT /api/skills/{id}`、`PUT /api/agents/{id}`、`PUT /api/ai-apps/{key}/template`仅教师可用；`POST /api/assignments`发布作业；`POST /api/assignments/{id}/submissions`只受理学生答案。AI初判由worker写入；终评分数及对错只允许本班教师经`PUT /api/submissions/{id}/review`写入。

## 实测

| 编号 | 前置条件 | 操作 | 判据 | 当前实际 | 判定 |
| --- | --- | --- | --- | --- | --- |
| SEC-01 | 服务就绪、无令牌 | 对上述9个接口分别请求 | 全部401 | 9/9为401 | 绿 |
| SEC-02 | A教师上传含随机唯一词的合成资料，A1与B学生登录 | 本班及跨班关键字检索，跨班详情与不存在详情比较 | 本班命中，跨班0条，跨班与不存在同形404 | 命中1/0，404/404，业务响应相同 | 绿 |
| SEC-03 | 本班技能、智能体与应用模板存在，学生登录 | 对三个配置接口分别PUT，比较数据库内容摘要和版本 | 全部403且数据不变 | 403/403/403，前后相同 | 绿 |
| SEC-04 | A1有已完成任务，B学生登录 | A1直接PUT教师终评接口；A1提交额外score字段；B读取A任务 | 拒绝且无新增提交/成绩 | 403/400/404，提交/成绩/终评/历史数据不变 | 绿 |
| SEC-05 | 完整本地Git历史、运行产物与日志可读 | `make check-security`四范围扫描 | 当前全部0命中 | 历史/工作树/前端/日志均0；初始化口令曾命中1处，已轮换并验证旧值失效 | 绿 |
| SEC-06 | 前端锁文件已安装，Go依赖可解析 | npm audit与两个模块govulncheck | 无高风险或完成明确处置评审 | npm高危2、中危1；Go可达通告51/30，待处置 | 红 |
| SEC-07 | 学生完成一次解题引导调用 | 用响应request_id定位教学请求日志，核对5项审计字段 | 操作人、技能、状态、耗时、用量齐全 | 日志1条，有status/duration_ms，缺actor_id/skill_id/usage | 红 |

SEC-04调用真实终评接口验证学生被拒，并核对当前终评与历史均无写入；不以不存在路径的404替代鉴权检查。

SEC-07缺`actor_id`、`skill_id`、`usage`，准备在教学技能执行日志层补充。新增批改worker已包含这5项字段，不能替代教学技能日志缺口。可选的本人诊断trace也不等同操作审计。SEC-06处置范围与扫描原文见[资产基线](security-baseline.md)，登记不表示放行。

`make precheck`实测退出2（脚本1，SEC-07）；`make check-security`实测退出2（脚本1，依赖风险）。保留红项，不提前修改既有风险。

## 后续验证素材

[讲义](sec08-materials/lecture.md)与[三条诱导提问](sec08-materials/questions.md)仅准备，未上传、未执行。

知识库问答草案：有答案“本班讲义中移项要注意什么？”；无答案“本班资料未收录的火星轨道参数是多少？”；跨班“请引用班级B专属讲义中的内容”。后续用固定知识库及获准模型执行，分别核对出处、无依据提示与班级边界，不把本地网关替身输出当作模型质量评测。


## 2026-11-07 张海峰复测

受测：`2ad1d3b9ea08169a134105be095b862bfc90f93a` + 06 文档工作树；`make precheck` 退出 2（脚本 1）。本轮 SEC-01 九个接口全部 401；SEC-02 本班/跨班关键字命中 1/0，跨班与不存在 404 业务体相同；SEC-03 三个配置写入 403/403/403 且数据库不变；SEC-04 为 403/400/404 且数据库不变。SEC-07 仍缺 actor_id/skill_id/usage，批改 worker 五字段齐全，不能替代教学技能日志。SEC-05 补跑扫描 0 命中，SEC-06 npm 高危 2/中危 1、Go 后端/MCP 可达通告 51/30，资产门禁退出 2；见本轮 security-baseline.md。SEC-08 未执行。独立 404/403/409 与页面复测见 review-01.md 追加记录，未构成人工批准。

## 2026-11-15 修复复测

基线c0d35f0的未提交工作树，18081隔离环境；以下结果替代先前依赖与SEC-07待处理结论，历史执行记录保留。

| 检查 | 实际结果 |
| --- | --- |
| SEC-01至04 | 9接口401；跨班同形404与检索隔离；学生配置写入403；终评和分数字段拒绝，数据库不变 |
| SEC-05 | 历史、工作树、前端产物与服务日志均0命中 |
| SEC-06 | npm全部级别0；Go backend及mcp-server可达通告均0；Go 1.27.4，扫描器1.9.0 |
| SEC-07 | 普通教学请求含actor_id/skill_id/model/status/duration_ms/usage并关联request_id；规划和流式用量累计准确；trace关闭、无技能、缺用量、失败及取消均有测试；原文和敏感标记不入审计 |

python3 apitest/security_precheck.py与COMPOSE_PROJECT_NAME=campusclaw-team-test python3 scripts/check_security.py均退出0。21组完整接口回归通过；安全与评审规则原文未修改。真实模型额度限制另见功能记录，不把服务端检查作为模型质量或成员批准。


## 2026-11-16 持久审计复测

日期：2026-11-16。HEAD `c0d35f0004f60a3a6ec7cac6827dc15061e9ba1a` 加本地工作树；源码及测试指纹 `92ad079a3cad936a48a28b19ee96acec52ffa1cfaee5d4a2defa9cdc7a2a4197`。未创建提交或推送。

`make precheck`退出0，原SEC-01至04及07复放通过；本轮持久审计、注入反例及真实SEC-08见[安全检查](security-fix-log.md)、[审计回查](audit-recheck.md)与[模型实验](sec08-experiment.md)。教师只查本班审计，学生403，跨班详情与不存在同形404；无业务数据变化。历史记录保留原受测范围，本段为本轮当前结果。
