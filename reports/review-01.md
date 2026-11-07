# 自查与交叉评审

> 来源记录：以下 2026-11-05 内容随 `codex/homework-teacher-review@df755cc` 交接导入，保留原受测提交与执行范围，不代表张海峰本次独立复测。2026-11-07 的集成与独立复测结果见 [review-01.md 的追加记录](review-01.md#2026-11-07-张海峰集成与独立复测)。

日期：2026-11-05。范围：`2026-11-04-async-grading`，A1至A11。当前记录为实现自查和自动检查；非作者逐文件评审、独立复现至少2项、异机启动及主干保护生效验证均**待完成**，不得据此合入main或标记正式版本。

## 七维自查

| 维度 | 判据与检查对象 | 实际结果 |
| --- | --- | --- |
| 需求覆盖率 | A1至A5、A7至A11的发布、受理、查询、停启、重投与失败场景；A6的检查及评审 | 功能场景已实现并经隔离回归；A6人工评审部分待完成 |
| 需求一致性 | AI仅初判对错并反馈，教师终评分数及对错；提交不等模型；不生成七项候选扩展 | 与规约一致；无既有同步批改耗时对比数据，不作虚构对比 |
| 失败路径 | 401/403/404/409、原子写失败、503恢复、永久失败、上限失败 | 用例已通过；重试3次、永久失败1次，死信事件持久化 |
| 查询性能 | 班级/归属过滤、分页、到期事件索引、网络调用在事务外 | 作业默认20上限100；任务主键查询；阻塞模型期间查询及提交仍小于1秒 |
| 重复代码 | 复用会话鉴权、API客户端及测试Context | 新增模块与现有协议衔接；未另造认证或评分客户端 |
| 安全风险 | 受保护接口、跨班、配置写入、密钥、依赖、审计 | 权限与当前密钥扫描绿；依赖与教学技能日志红，未放行 |
| 并发/事务/一致性 | 唯一键、结果事务后ACK、租约围栏、原子受理 | 真实MySQL/RabbitMQ用例通过，详见三份risk记录 |

执行：`make check`中Go vet/test/build、前端9项测试及构建、严格规约、OpenAPI、五组接口回归均通过；最终资产门禁红，make退出2。受测HEAD为`ba61ce5`并含本次回归与文档更改。`make precheck`退出2，保留真实日志缺口。13个开发技能格式、24条相对链接、44个空值配置键及无二进制大文件检查通过。

## 文件结论

“可用”仅指已完成的实现自查，仍须非作者评审；“待修改”项未获放行。生成契约与整合文件由对应构建/行为检查覆盖，人工评审范围仍应涵盖完整PR diff。运行平台整合是作业链路的前置依赖，按task分提交；生成OpenAPI与锁文件单列，不能用文件数量替代语义评审。

| 文件 | 结论与依据 |
| --- | --- |
| backend/internal/grading/models.go | 可用；学科/Unicode边界、幂等键与终评字段有单元测试 |
| backend/internal/grading/http.go | 可用；严格JSON、角色、参数与只读查询，A1/A2/A4用例 |
| backend/internal/grading/store.go | 可用；SQL集中、班级与本人过滤、受理三写同事务 |
| backend/internal/grading/migrate.go | 可用；迁移锁、成功后记版本，重复迁移2次通过 |
| backend/internal/grading/migrations/001_async_grading.sql | 可用；提交、任务、结果与发件箱唯一约束及索引 |
| backend/internal/grading/worker_store.go | 可用；短事务领取、run_token围栏，旧执行者不能覆盖 |
| backend/internal/grading/broker.go | 可用；持久队列/消息、确认超时边界、手工ACK |
| backend/internal/grading/gateway.go | 可用；不跟随重定向、结构校验、错误分类、不泄漏原文 |
| backend/internal/grading/prompts/verdict-v2.txt | 可用；固定版本且只给对错初判；仅本地替身验证，真实模型质量待独立评测 |
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
| frontend/src/pages/Homework.tsx | 可用；分学科发布/作答/批改、四状态、参考答案隐藏、刷新恢复 |
| frontend/src/App.tsx | 可用；按角色接入作业页，保留现有入口 |
| frontend/src/pages/Login.tsx | 可用；登录入口与接口文档链接 |
| frontend/src/styles.css | 可用；A9-S5的Safari实页、375/768/1280宽度、深浅主题及文字对比度核对通过 |
| backend/internal/grading/review_store.go | 可用；当前终评/历史同事务，版本锁及有效错题投影 |
| backend/internal/grading/review_http.go | 可用；本班教师终评、学生本人查询、参数与状态拒绝 |
| backend/internal/grading/migrations/002_teacher_review.sql | 可用；增量结构，旧数值保留，新AI不写数值 |
| frontend/src/components/HomeworkParts.tsx | 可用；学科选择、AI初判与教师终评分别展示，false/0不被忽略 |
| frontend/src/components/ReviewDialog.tsx | 可用；终评分数及对错、参考答案、错误提示与焦点管理；A9-S3/S5取消恢复与横排标签通过 |
| frontend/src/pages/Mistakes.tsx | 可用；按学科读取有效错题，空/错误/加载状态 |
| apitest/cases/api/submissions/review/PUT.py | 可用；四组合、并发/事务回滚、越权、晚到AI及最新作答 |
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

结果：待真实独立复现后填写；当前没有人工批准或异机启动证据；GitHub实查main的protected=false，主干保护尚未启用。安全红项需后续变更处置并复测，不能仅靠本报告解除门禁。


2026-11-05追加自查：A7至A10的学科隔离、严格布尔初判、教师终评和错题流转已验证。原有扫描红项仍单独处置；本次变更保持待审，不以自动检查替代非作者批准。

A11自查：统一语文、数学、英语、科学、思想品德，资料另含其他；003迁移将原物理/化学作业归入科学，终评及历史保留，科学错题仍为3/10。学科枚举、班级隔离及迁移重跑通过。

A11-S2（2026-11-05）：受测HEAD为`6967260141496ea4475c1349aac8342fb1beebac`，含筛选修订。`docker compose build api web`（含Go vet/test/build）、`make check-frontend`（9项）、`python3 apitest/run_all.py --case assignments`（2组）、OpenSpec严格校验与OpenAPI一致性检查均退出0。作业和错题仅五项筛选，旧all缓存恢复数学，接口all/非法值返回400且无写入。Chrome教师端和Safari学生端实际核对五项顺序、科学记录及资料其他分类；资料总览入口为本班资料。既有安全红项和人工评审状态保留。

A12（2026-11-05，受测HEAD为`f2b39047d632f70042d639e05779926a616859ad`及本次工作树）：Go镜像检查、前端9项与`python3 apitest/run_all.py --case initial-judgment`退出0。独立测试环境验证网络重试复用任务、重新提交创建新任务、历史更新保留旧结果/终评、失败无写入及重复更新0项。真实模型course-chat以grading-verdict-v3完成相反数等价作答初判和同答案新任务；8份旧初判更新成功，旧结果与3份教师终评及历史摘要一致，重复更新0项。密钥仅在本机配置，评测内容为合成题。

A13（2026-11-05，受测HEAD为`ab776461148fe407b4afb1aece2517dcad3314a3`及本次工作树）：Go构建/单元、前端9项、`python3 apitest/run_all.py --case explanation`及`--case tasks`退出0。新增接口/存储/模型适配和逐题组件按A13逐段自查：授权先于模型调用，SQL仅在存储层，模型不接收参考答案；并发200/429、改判与新作答409、取消1次、非法响应503，原作答无变更。Safari实测逐题按钮、加载及真实讲解展示；人工交叉评审仍待完成。

模型评测：`python3 apitest/evaluate_model.py --live`退出0，course-chat、temperature=0、初判max_tokens=512、讲解max_tokens=768，模板grading-verdict-v3 / mistake-explanation-v1。首轮18/20；细化行为选择提示后保持输入和参数重放，最终20/20。正确题只验初判，15份错题另验讲解；逐项预期见apitest/fixtures/model-eval.json，前后实际输出保存在本机output/model-eval-before.json与model-eval.json。

| ID | Expected / actual | Verdict | Explanation |
| --- | --- | --- | --- |
| N01 | true / true | PASS | SKIP (correct) |
| N02 | false / false | PASS | PASS |
| N03 | true / true | PASS | SKIP (correct) |
| N04 | false / false | PASS | PASS |
| N05 | true / true | PASS | SKIP (correct) |
| N06 | false / false | PASS | PASS |
| N07 | true / true | PASS | SKIP (correct) |
| N08 | false / false | PASS | PASS |
| N09 | true / true | PASS | SKIP (correct) |
| N10 | false / false | PASS | PASS |
| R01 | false / false | PASS | PASS |
| R02 | false / false | PASS | PASS |
| R03 | false / false | PASS | PASS |
| R04 | false / false | PASS | PASS |
| R05 | false / false | PASS | PASS |
| R06 | false / false | PASS | PASS |
| R07 | false / false | PASS | PASS |
| R08 | false / false | PASS | PASS |
| R09 | false / false | PASS | PASS |
| R10 | false / false | PASS | PASS |

A9/A13-S3最终核对（2026-11-05，受测HEAD为`8dd1984`及本次工作树）：Chrome在375/768/1280宽度下确认最终判定两项处于同一横排、文字不换行，视口无横向溢出，关闭后焦点返回原按钮。Go vet/test/build、前端9项、OpenSpec、41个接口路径一致性、19个相对链接及七组完整API回归通过。并发用例以替身放行点协调，测试计数按配置批次隔离。

`make check-security`与`make precheck`仍退出2：密钥扫描0命中；npm high=2、moderate=1，Go可达通告51/30；教学技能审计仍缺actor_id、skill_id、usage。新增初判与讲解验证不替代既有安全红项或非作者评审。

5.11规约自查（2026-11-05，基于`963567b`）：四件套补清单题文本作业边界、学生接口参考答案隔离及A9-S3/S4/S5的操作、失败和视觉判据；对照Homework.tsx、ReviewDialog.tsx及review_http.go/review_store.go核对字段、取消无写入、输入保留和版本冲突。严格规约校验与`git diff --check`退出0。A9-S5的Safari与颜色复核列入5.12；既有自动检查不替代未完成的成员复测。

5.12界面自查（2026-11-05，基于`e4c2a33`及本次工作树）：ReviewDialog.tsx以独立文字span承载对错标签，styles.css限定两列内容宽度与18px单选框。Safari在18080实页确认深浅主题横排、正确绿色/错误红色；Chrome在375/768/1280px、深浅主题共六组核对，两标签顶部差0px、高44px、无横向溢出。浅色文字对比度为7.18/7.34:1，深色9.48/9.01:1。改选错误后取消、关闭或Escape，重新打开仍为正确、1分、终评版本1；焦点返回修改终评入口。`make check-frontend`（类型/构建与9项测试）、`docker compose build web`退出0，仅更新本地web；本次未改接口、模型、提示词或数据结构，既有安全红项与成员复测保持原状态。


## 2026-11-07 张海峰集成与独立复测

执行者：张海峰，本人授权由 AI 辅助实际执行。Git 作者/提交者为 `张海峰 <Hai-Story@users.noreply.github.com>`，GitHub 登录与提交归属均为 `Hai-Story`。这不是其他成员的人工批准；04/05 的集成作者仍需其他成员评审。对 02/03 后端的独立请求不复用交接包中 `run_all.py` 的用例函数。

固定基线：`c9c1548176b40e9d5f956efe294b66332bab2856`，前序链为 `09f787a → 7d82fa1 → 47e672f → c9c1548`。来源仍为 `codex/homework-teacher-review@df755cc`。04 提交 `77bbce9b3f838531f4981f895df8ec11616cae73`、05 提交 `2ad1d3b9ea08169a134105be095b862bfc90f93a` 已分别推送协作分支。06 受测对象为该 05 提交及 06 文档工作树；06 没有修改应用源码、依赖或测试逻辑。

环境：2026-11-07（Asia/Shanghai），macOS/arm64，Node.js 24.19.0、npm 11.17.0、Python 3.9.6、Docker 29.8.0 / Compose v5.5.1、OpenSpec 1.13.0；Go 容器为 1.23.12 linux/arm64。隔离项目 `campusclaw-team-test`、本机 18081、独立数据卷、MySQL/RabbitMQ 及本地模型替身；无真实学生数据和计费模型调用。新生成的测试凭据不进报告/版本库。

### 分包命令与实际结果

| 包 | 作用 | 实际验证 | 退出码 |
| --- | --- | --- | --- |
| 04 | 30 个路径（含删除旧前端 README），师生页面与终评/错题交互 | `npm --prefix frontend ci`；`npm --prefix frontend run build`；OpenSpec strict；暂存差异检查 | 均 0 |
| 05 | 51 个路径（含删除旧接口测试 README），自动检查、测试和网关替身 | `make check-go`；`make check-frontend`；Python 源文件 compile；OpenAPI check；暂存差异检查 | 均 0；前端 9/9 |
| 06 | 12 个路径，历史证据、说明与任务状态，追加本次复测 | Compose config；隔离启动；完整检查及补测；独立 API 请求；Chrome 实页；链接与资产检查 | 见下表，不把非零写成通过 |

| 本次命令/步骤 | 实际结果 |
| --- | --- |
| `python3 scripts/dev_env.py --test` | 退出 0；生成全新测试配置，不覆盖已有配置 |
| `docker compose --env-file .env.test --profile test config --quiet` | 退出 0 |
| `docker compose --env-file .env.test --profile test up --build -d` | 首次退出 1：Docker Hub 获取 alpine 认证令牌连接超时，未到应用编译；代理端口 7897 可用且认证请求 HTTP 200，命令级 HTTP_PROXY/HTTPS_PROXY 预拉 alpine/nginx 后同命令重试退出 0；未改系统/Docker 全局设置 |
| `curl http://127.0.0.1:18081/health` / `/live` | 均 HTTP 200；health 的 search/storage/tools 均 up |
| `COMPOSE_ENV_FILES=.env.test make check` | 退出 2；Go、前端、规约、OpenAPI、差异、7/7 API 组均通过；末尾资产扫描在默认 npmmirror 的 npm 审计请求失败，不能判资产检查已完成 |
| `npm_config_registry=https://registry.npmjs.org COMPOSE_ENV_FILES=.env.test make check-security` | 补跑失败阶段，make 退出 2 / Python 退出 1；扫描完整完成，依赖门禁为红，摘要见 security-baseline.md |
| `make precheck` | make 退出 2 / Python 退出 1；SEC-01 至 SEC-04 绿，SEC-07 红，缺 actor_id/skill_id/usage；批改 worker 的五项字段齐全，不能代替教学技能日志 |
| `python3 ../independent-retest.py`（本次工作目录外的独立检查脚本） | 退出 0；下列三个场景均通过；本报告给出独立步骤与数据，便于不依赖脚本重放 |
| Playwright CLI Chrome 实页操作 | 退出 0；三档宽度/深浅主题、取消、确认、学生重新作答/刷新/错题/讲解均执行；细节如下 |
| `openspec validate 2026-11-04-async-grading --strict`；`python3 scripts/generate_openapi.py --check`；`git diff --check` | 均退出 0 |
| 相对文档链接与文件资产检查 | 28 条链接目标存在；三包共 93 个路径，无 NUL 二进制或超过 1 MiB 的文件 |

最终 06 暂存的 12 个文件另做 gitleaks v8.24.3 定向扫描，退出 0、无命中；`git diff --cached --check` 退出 0。

Go 默认测试中的 `TUTOR_INTEGRATION` / `MCP_INTEGRATION` 专用数据库集成测试未启用，明确为 SKIP；上表的 7 组接口回归确实使用运行中的 MySQL/RabbitMQ，不能与 Go 被跳过的测试混为一谈。初次完整门禁没有整体通过；补跑资产阶段也仍是红项，没有删除断言或放宽门禁。

### 独立复测：手段、对象、判据、结果

新建 A 班数学作业，A1 提交合成错误答案并等待完成；B1 读取该作业和不存在 ID `9223372036854775807`。之后 A1 PUT 终评，再由教师成功写入 0 分/false/expected_version=0，最后教师以相同旧版本写入 10 分/true。每次拒绝前后查询 `grade_reviews` 和 `grade_review_history`。请求复用仓库 Context 的隔离守卫及登录/SQL 传输，但判断与执行脚本独立于交接的用例函数。

| 场景 | 判据 | 实际结果 |
| --- | --- | --- |
| A1-S1 跨班读取 | 跨班与不存在均同形 404，业务响应完全相同 | 404 / 404；两者均为 `{"error":"资源不存在"}` |
| A8-S2 学生调用终评 | 403 且当前终评与历史均不变 | 403；前后历史数 0，当前终评无行 |
| A8-S2 旧版本终评 | 教师首次成功，旧版本 409，无覆盖、无额外历史 | 200 后 409；前后均为 score=0/correct=0/version=1，历史数 1 |

独立输出（仅合成数据和脱敏结果）：

```json
{
  "A1-S1_cross_class_404": {
    "cross": 404,
    "missing": 404,
    "body_equal": true,
    "body": {
      "error": "资源不存在"
    }
  },
  "A8-S2_student_403": {
    "status": 403,
    "database_unchanged": true,
    "before": [
      [
        "0"
      ]
    ],
    "after": [
      [
        "0"
      ]
    ]
  },
  "A8-S2_stale_version_409": {
    "initial_status": 200,
    "stale_status": 409,
    "database_unchanged": true,
    "before": [
      [
        "0",
        "0",
        "1"
      ],
      [
        "1"
      ]
    ],
    "after": [
      [
        "0",
        "0",
        "1"
      ],
      [
        "1"
      ]
    ]
  }
}
```

复测结束已清理本次创建的作业/提交及测试会话；不操作生产数据。

### 异步与页面复测

接口运行器实际发现并运行 7 组：assignments POST、subjects GET、submissions POST、explanation POST、initial-judgment POST、review PUT、tasks GET，均 PASS。A2-S1 每档恰好 20 次，2 秒/4 秒替身的响应 P95 为 3.935/4.796ms，后台执行 2007.828/4003.987ms。任务恢复、过期执行者丢弃、消费者强制退出后 attempts=2、结果唯一 1、503 重试 3 次、401 永久失败 1 次、2 条死信事件 published、迁移重跑 2 次均在本轮实际观察；终评并发为 200/409，历史失败回滚保持一致。

Chrome 的教师终评在 375/768/1280px 和深浅主题共 6 组，标签顶部差均为 0px，点击高度 44px，单选框宽 18px，文字行高 22.4px 且为单行，页面无横向溢出。正确/错误文字对比度：浅色 7.18/7.34:1，深色 9.48/9.01:1。Escape 后焦点回原入口；另测取消与关闭，监听到的终评 PUT 为 0，SQL 的当前终评/历史为 0/0。1280×500 时侧栏可独立滚动，滚到底后账户操作可见。

实际 UI 链路：教师打开提交并确认正确、8/10，返回 200 / version=1；学生页面不显示参考答案，显示教师正确与 8/10，题目移出错题；学生重新提交相同错误答案返回 202、新 submission_id，刷新恢复 AI 初判且无教师分数，新错题可见；点击讲解返回 200 并显示讲解。取消测试与实际确认分别记录，未将确认写入混称“取消无写入”。UI 合成数据与测试登录均已清理。替身反馈只是测试协议输出，不证明真实模型理由质量。

Safari 本机复测、真实模型 20 条评测及异机启动本轮未执行，不能拿 Chrome 或替身冒充。5.8/5.9/5.12 保留未完成，来源的既有历史结论仍可追溯。

### 本轮七维核查与待审事项

| 维度 | 本次核查对象与结论 |
| --- | --- |
| 需求覆盖 | A1-A5/A7-A8/A11-A13 的接口分支由 7 组真实服务回归覆盖；A9 的 Chrome 主路径与布局另测；Safari/真实模型未覆盖 |
| 一致性 | ReviewDialog 只提交教师 score/correct/feedback/expected_version；学生无参考答案，重新提交不覆盖历史；不扩展 Non-goals |
| 失败路径 | 401/403/404/409、受理/终评事务失败、503/401 模型替身故障、讲解取消/429 均有真实回归；网络/审计首次失败单列 |
| 查询性能 | grading store/review_store 的归属过滤/分页与事务外模型调用静态核对；异步响应与阻塞场景实测；未声明一般列表 100 次 p95 基准或生产性能 |
| 重复代码 | 前端复用 homework API、ResultPanel、SubjectPicker；测试复用 Context 的隔离和清理；独立复测单独判断 |
| 安全风险 | SEC-01..04 本轮通过；SEC-05 四范围 0 命中；SEC-06 高危依赖/Go 可达通告、SEC-07 教学技能日志仍红；SEC-08 素材未执行 |
| 并发/事务 | store.Submit 三写同事务、review_store.Review 锁行/比版本/当前与历史同事务、runner 在状态事务后 ACK 静态核对；真实并发、回滚及过期执行场景通过 |

GitHub 本轮只读核对 `main` 为 `protected=false`，未修改分支保护。4.1 仅完成本次辅助独立复测部分，非作者逐文件人工批准、异机启动、红项处置/正式风险接受和主干保护仍未齐备；4.2 未合并、未归档、未打标签。3.3 的团队 PR 仍待郑靖炫按交接文档发起。本轮只向已授权协作分支提交与推送，不代其他成员签字。

规则引用：harness/rules/03-security.md（跨班同形与依赖红项）、04-testing.md（真实结果与 SKIP）、08-review.md（七维核查、非作者批准与完成定义）；页面对应 01-product.md 与 A9，事务对应 02-backend.md、07-data.md 与 A2/A8，替身范围对应 06-ai.md 和设计 D5/D8。

### 三包文件集成结论

下表“可用”限定为本次集成审阅与相应自动/实页检查范围，仍需团队逐文件人工语义审查，不表示风险已获放行。规则、门禁、任务状态及历史证据改动尤其须人工确认。各包是从同一已存在来源重放的完整交接边界（04 为 1402 行新增、05 为 3719 行新增，含既有界面/测试整合），按用户明确要求分别提交，未把手写新增需求混入来源导入。

| 包 | 文件 | 结论与范围 |
| --- | --- | --- |
| 04 | `frontend/README.md` | 可用；移除旧占位说明，说明归并根 README |
| 04 | `frontend/index.html` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/App.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/agent-suggestions.ts` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/ai-apps.ts` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/citations.ts` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/components/HomeworkParts.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/components/Icon.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/components/MistakeExplanation.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/components/ReviewDialog.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/components/Toast.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/components/TraceDetails.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/conversation-lifecycle.ts` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/conversation-time.ts` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/library.ts` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/main.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/pages/AIApps.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/pages/Ask.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/pages/Homework.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/pages/Login.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/pages/MCP.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/pages/Materials.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/pages/Mistakes.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/pages/Teacher.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/pages/Tutor.tsx` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/skill-tools.ts` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/stream-state.ts` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/src/styles.css` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/tsconfig.json` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 04 | `frontend/vite.config.ts` | 可用；前端类型/构建覆盖，作业链路另有 Chrome 实测；人工审阅待完成 |
| 05 | `Makefile` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/README.md` | 可用；移除旧占位说明，说明归并根 README |
| 05 | `apitest/cases/api/assignments/POST.py` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/cases/api/assignments/subjects/GET.py` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/cases/api/submissions/POST.py` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/cases/api/submissions/explanation/POST.py` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/cases/api/submissions/initial-judgment/POST.py` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/cases/api/submissions/review/PUT.py` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/cases/api/tasks/GET.py` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/common.py` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/evaluate_model.py` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/fixtures/model-eval.json` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/gateway.mjs` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/run_all.py` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `apitest/security_precheck.py` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/auth/roles_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/chunking/chunking_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/config/config_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/db/material_seed_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/grading/explanation_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/grading/gateway_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/grading/models_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/materials/mutations_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/mcp/integration_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/mcp/protocol_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/retrieval/stream_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/retrieval/trace_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/runtimehttp/http_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/tutor/apps_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/tutor/delete_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/tutor/evidence_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/tutor/integration_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/tutor/manifest_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/tutor/orchestration_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/tutor/prompt_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/tutor/skill_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/tutor/templates_integration_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/tutor/templates_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/tutor/timestamps_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `backend/internal/tutor/trace_integration_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `frontend/tests/auth.test.mjs` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `frontend/tests/citations.test.mjs` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `frontend/tests/conversation-time.test.mjs` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `frontend/tests/conversations.test.mjs` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `frontend/tests/grading.test.mjs` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `frontend/tests/library.test.mjs` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `frontend/tests/skill-manifest.test.mjs` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `frontend/tests/sse.test.mjs` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `frontend/tests/tools.test.mjs` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `mcp-server/main_test.go` | 可用；对应 Go/前端/接口检查通过；专用 Go 数据库集成测试 SKIP 另列，人工审阅待完成 |
| 05 | `scripts/check_security.py` | 可用；真实四范围扫描及依赖红项非零；默认 registry 故障以命令级设置补测 |
| 06 | `README.md` | 可用；历史来源已标注，当前复测另记；不等同人审批准 |
| 06 | `docs/platform-overview.md` | 可用；历史来源已标注，当前复测另记；不等同人审批准 |
| 06 | `openspec/changes/2026-11-04-async-grading/tasks.md` | 可用；PR/真实模型/Safari/成员评审与合入仍有未完成任务 |
| 06 | `reports/async-response.md` | 可用；历史来源已标注，当前复测另记；不等同人审批准 |
| 06 | `reports/review-01.md` | 可用；历史来源已标注，当前复测另记；不等同人审批准 |
| 06 | `reports/risk-authz.md` | 可用；历史来源已标注，当前复测另记；不等同人审批准 |
| 06 | `reports/risk-concurrency.md` | 可用；历史来源已标注，当前复测另记；不等同人审批准 |
| 06 | `reports/risk-transaction.md` | 可用；历史来源已标注，当前复测另记；不等同人审批准 |
| 06 | `reports/sec08-materials/lecture.md` | 可用；历史来源已标注，当前复测另记；不等同人审批准 |
| 06 | `reports/sec08-materials/questions.md` | 可用；历史来源已标注，当前复测另记；不等同人审批准 |
| 06 | `reports/security-baseline.md` | 待修改（风险处置）；本轮扫描证据真实，依赖红项未修复/未放行 |
| 06 | `reports/security-precheck.md` | 待修改（风险处置）；本轮 SEC-07 缺审计字段，未放行 |
