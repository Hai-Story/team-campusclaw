# 自查与交叉评审

日期：2026-11-04。范围：`2026-11-04-async-grading`，A1至A6。当前记录为实现自查和自动检查；非作者逐文件评审、独立复现至少2项、异机启动及主干保护生效验证均**待完成**，不得据此合入main或标记正式版本。

## 七维自查

| 维度 | 判据与检查对象 | 实际结果 |
| --- | --- | --- |
| 需求覆盖率 | A1至A5的发布、受理、查询、停启、重投与失败场景；A6的检查及评审 | 功能场景已实现并经隔离回归；A6人工评审部分待完成 |
| 需求一致性 | 客观题精确匹配评分，模型仅反馈；提交不等模型；不生成七项候选扩展 | 与规约一致；无既有同步批改耗时对比数据，不作虚构对比 |
| 失败路径 | 401/403/404/409、原子写失败、503恢复、永久失败、上限失败 | 用例已通过；重试3次、永久失败1次，死信事件持久化 |
| 查询性能 | 班级/归属过滤、分页、到期事件索引、网络调用在事务外 | 作业默认20上限100；任务主键查询；阻塞模型期间查询及提交仍小于1秒 |
| 重复代码 | 复用会话鉴权、API客户端及测试Context | 新增模块与现有协议衔接；未另造认证或评分客户端 |
| 安全风险 | 受保护接口、跨班、配置写入、密钥、依赖、审计 | 权限与当前密钥扫描绿；依赖与教学技能日志红，未放行 |
| 并发/事务/一致性 | 唯一键、结果事务后ACK、租约围栏、原子受理 | 真实MySQL/RabbitMQ用例通过，详见三份risk记录 |

执行：`make check`中Go vet/test/build、前端9项测试及构建、严格规约、OpenAPI、三组接口回归均通过；最终资产门禁红，make退出2。受测HEAD为`fe0b754`并含本次待提交更改。`make precheck`退出2，保留真实日志缺口。13个开发技能格式、24条相对链接、44个空值配置键及无二进制大文件检查通过。

## 文件结论

“可用”仅指已完成的实现自查，仍须非作者评审；“待修改”项未获放行。生成契约与整合文件由对应构建/行为检查覆盖，人工评审范围仍应涵盖完整PR diff。运行平台整合是作业链路的前置依赖，按task分提交；生成OpenAPI与锁文件单列，不能用文件数量替代语义评审。

| 文件 | 结论与依据 |
| --- | --- |
| backend/internal/grading/models.go | 可用；Unicode边界、幂等键与确定性评分有单元测试 |
| backend/internal/grading/http.go | 可用；严格JSON、角色、参数与只读查询，A1/A2/A4用例 |
| backend/internal/grading/store.go | 可用；SQL集中、班级与本人过滤、受理三写同事务 |
| backend/internal/grading/migrate.go | 可用；迁移锁、成功后记版本，重复迁移2次通过 |
| backend/internal/grading/migrations/001_async_grading.sql | 可用；提交、任务、结果与发件箱唯一约束及索引 |
| backend/internal/grading/worker_store.go | 可用；短事务领取、run_token围栏，旧执行者不能覆盖 |
| backend/internal/grading/broker.go | 可用；持久队列/消息、确认超时边界、手工ACK |
| backend/internal/grading/gateway.go | 可用；不跟随重定向、结构校验、错误分类、不泄漏原文 |
| backend/internal/grading/prompts/feedback-v1.txt | 可用；固定版本且不决定成绩；仅本地替身验证，真实模型质量待独立评测 |
| backend/internal/grading/settings.go | 可用；必填、数值与租约大于超时校验 |
| backend/internal/grading/runner.go | 可用；外部调用无事务占用，结果提交后ACK，重试/死信有界 |
| backend/cmd/server/main.go | 可用；复用服务端身份中间件挂载作业接口 |
| backend/cmd/worker/main.go | 可用；独立消费、受控迁移与指标入口，不公开HTTP改分 |
| backend/cmd/relay/main.go | 可用；独立投递进程、停止信号与重连 |
| backend/internal/runtimehttp/http.go | 可用；随机请求标识、脱敏JSON日志与探针 |
| backend/internal/config/config.go | 可用；缺必填或非法范围启动失败 |
| backend/internal/apidocs/apidocs.go | 可用；本地文档与OpenAPI读取，不载入外部脚本 |
| backend/internal/apidocs/static/index.html | 可用；接口列表与文本渲染 |
| backend/internal/apidocs/static/openapi.json | 可用；生成契约，检查全部注册路径与注释一致 |
| scripts/api_schemas.json | 可用；任务/受理/结果字段与实际响应一致 |
| scripts/generate_openapi.py | 可用；生成并检查路由漂移 |
| frontend/src/grading.ts | 可用；稳定幂等键、持久指针校验、终态和离页中止轮询 |
| frontend/src/pages/Homework.tsx | 可用；四状态、标准答案隐藏、刷新恢复与文本安全渲染 |
| frontend/src/App.tsx | 可用；按角色接入作业页，保留现有入口 |
| frontend/src/pages/Login.tsx | 可用；登录入口与接口文档链接 |
| frontend/src/styles.css | 可用；375/768/1280宽度实际查看 |
| apitest/common.py | 可用；隔离配置守卫、私有凭据、本次合成数据清理 |
| apitest/run_all.py | 可用；按URL/方法发现用例，失败非零并保存实际观测 |
| apitest/cases/api/assignments/POST.py | 可用；权限、冲突、回滚及查询无副作用 |
| apitest/cases/api/submissions/POST.py | 可用；两档20次与停止relay后恢复 |
| apitest/cases/api/tasks/GET.py | 可用；消费者停启、中断、租约失效、重试与死信 |
| apitest/gateway.mjs | 可用；仅test profile内部网关替身，无宿主端口 |
| apitest/security_precheck.py | 可用；实际权限与日志字段核对，红项非零，不执行SEC-08 |
| scripts/check_security.py | 可用；四范围全历史资产扫描与依赖审计，发现风险非零 |
| scripts/dev_env.py | 可用；私有文件随机生成，不覆盖已有配置 |
| docker-compose.yml | 可用；隔离卷与网络、仅web绑定本机、管理口令仅数据库挂载 |
| Makefile | 可用；Go/前端/规约/接口/资产串行门禁，不吞掉非零 |
| backend/go.mod | 待修改；受测Go链与依赖存在可达安全通告，见资产基线 |
| backend/Dockerfile | 待修改；构建链升级须另行验证后评审 |
| mcp-server/go.mod | 待修改；当前构建链存在可达安全通告 |
| mcp-server/Dockerfile | 待修改；与后端一并升级构建链并复测 |
| frontend/package-lock.json | 待修改；npm高危2、中危1，不能以锁定版本替代安全更新 |
| backend/internal/tutor/stream.go | 待修改；教学技能审计缺actor_id/skill_id/usage |

高风险四段记录：权限见[risk-authz.md](risk-authz.md)，并发见[risk-concurrency.md](risk-concurrency.md)，事务见[risk-transaction.md](risk-transaction.md)。规则、技能、规约、构建与门禁改动仍须人工语义审查；未发现需标为“风险无效”的新增功能实现。

## 交叉评审：待完成

手段：非作者在独立环境按README启动，不预先套用本记录结论。

对象：至少复现SEC-02与SEC-03；同时核对PR逐文件结论及安全红项处置。

判据：独立记录实际命令、输出和数据副作用；SEC-02得同形404与0条跨班结果，SEC-03得403且数据不变，与原判定一致。分别运行`make precheck`和`python3 apitest/run_all.py --case assignments`即可复现对应行为。

结果：待真实独立复现后填写；当前没有人工批准、异机启动或主干保护生效证据。安全红项需后续变更处置并复测，不能仅靠本报告解除门禁。
