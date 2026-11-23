# 张海峰交接复测记录

2026-11-23，受测提交 `a708dbf18fba28ee935c704f818068970b13a596`（第 19 步）。本机使用独立检出、Docker Desktop、新建的测试数据卷及本地模型替身。

## 提交与树核对

远端 `codex/learning-security-integration` 的第 15 步为 `026b8c0459c9118d8ce56a7ad8610da9244f2ab9`，树为 `e2c4c80e536f11d7d93b19b06f2b67d049a388bc`。在干净检出中创建了跟踪该远端分支的 `handoff-zhang-20261122`。Git 作者与提交者均为 `Hai-Story <Hai-Story@users.noreply.github.com>`。GitHub 返回的已登录账号为 `Hai-Story`，且将历史提交 `36152399` 中相同邮箱的作者和提交者都关联至该账号。

第 16–19 步逐项核对了补丁 SHA-256、前置树、暂存文件集合和目标树。每步的 `git apply --check --index` 与 `git diff --cached --check` 均退出 0，提交后的树与提交前的 `git write-tree` 相同。

| 步骤 | 本地提交 | 核实的目标树 |
| --- | --- | --- |
| 16 | `a87be8c3a38ce538ea6245f1caa1adfb7de259dc` | `c754a456815913379c85160e00c6d7261251a263` |
| 17 | `dd99ee72f40c8efbec2d53d1bc08604facad6b02` | `6ad5bb5ec1892b260b7c127d8be835aa12052899` |
| 18 | `d10059ed57c8bb87b95f06b6cd8bc5c67e1425b7` | `406d4ab7962238bd63b337d86b18e769aa47fb3f` |
| 19 | `a708dbf18fba28ee935c704f818068970b13a596` | `e18deda5909a8661ad196eb6eb5b783a6da7dd54` |

第 20、21 步分别核对了 SHA-256，按第 19 步的实际文件内容逐段匹配补丁上下文，在内存中计算变更后的 Git blob 与目录树。第 20 步得到 `16f031efa01859ff6b9781c07610995acf9656a6`，第 21 步得到 `23804aa94d1ab516e03d291ffbe736ce42c48761`，均与清单相符。两步均未提交。第 20 步的真实模型实验会自动尝试提取隐藏指令、越权修改评分并保存回复。此入口未集成或运行。

## 第 19 步的实际检查

使用仓库的测试配置生成器创建了本次专用测试配置。Compose 使用仓库规定的测试项目名和 18081 端口，另用 Compose 覆盖文件指定全新数据卷和网络，镜像标签为 `zhang-a708dbf`。Node 与 npm 在仅挂载项目前端的 `node:22-alpine` 容器中运行。全量检查第一次因前端依赖尚未就绪，在 TypeScript 构建处退出 2。按锁文件完成 `npm ci` 后重跑结果如下。

| 命令或探针 | 退出码 | 观测 |
| --- | ---: | --- |
| `docker compose --env-file .env.test --profile test up --build --wait --wait-timeout 180` | 0 | 启动 9 个本机隔离服务。随后请求 `/live` 和 `/ready`，均返回 HTTP 200 |
| `COMPOSE_ENV_FILES=.env.test make check` | 0 | Go vet、测试和构建通过，前端构建和 12 组测试通过，OpenAPI 路由和注解一致，24 组接口回归通过 |
| 上述命令中的 `make check-security` | 0 | 711 个可达历史 blob、工作树、运行中前端和隔离日志的扫描均为 0 命中。npm 漏洞计数为 0，两个 Go 模块的可达通告均为 0 |
| `make precheck` | 0 | SEC-01、02、03、04、07 均报告 GREEN |
| `openspec validate --all --strict` | 0 | 17 项通过，0 项失败 |

## 单独复现

| 项目 | 命令或操作 | 退出码及观测 |
| --- | --- | --- |
| 扫描器能拦截合成样本 | `SECURITY_PROVE_RED=1 make check-security` | 扫描器退出 1，make 退出 2。输出包含 `synthetic_fixture=true` 与 `gate=RED` |
| 审计列表访问控制 | 独立探针依次以匿名、测试学生、测试教师请求 `GET /api/audit-events?page_size=2` | 探针退出 0；依次返回 401、403、200，教师结果数量为 1 至 2 条 |
| 资料标题只作为文本渲染 | 在禁网 Node 容器中编译实际 `MaterialTitle.tsx`，调用 React 的 `renderToStaticMarkup` | 断言退出 0；输入中的 `<b>`、`</b>` 和 `&` 分别输出为 `&lt;b&gt;`、`&lt;/b&gt;` 和 `&amp;` |

从第 20 步单独取出两个审计接口测试，在隔离库运行其 `run(Context())`，均退出 0。审计落库不可用时业务写入收到 503，健康探针仍为 200。审计列表 5 次预热后测量 100 次，p95 为 6.31 毫秒。教学审计覆盖正常、失败、取消及用量缺失。

从第 21 步单独取出四个 Go 测试文件，在第 19 步源码副本上运行 `go test -count=1 ./internal/auditlog ./internal/auth ./internal/grading ./internal/securityaudit`，四个包通过，退出 0。

## 交付状态

原有检出未切分支或改动文件。本次实现分支保留在第 19 步。验收证据单独提交到本地 `handoff-zhang-20261122-evidence`，以便后续从第 19 步继续。未推送协作分支。

本次启动证据仅覆盖当前 Mac 上的 Docker。另一台机器的启动，以及第 20–21 步提交后在最终交付树上运行全量门禁，均待完成。
