# 第 20–21 步本机验证

2026-11-23，张海峰，GitHub 账号 `Hai-Story`。以协作分支第 19 步 `a708dbf18fba28ee935c704f818068970b13a596` 为基线，在本机独立克隆中使用 Docker Desktop、新建数据卷和仓库自带的测试模型网关。

第 20 步的两个审计接口用例通过，第 21 步的 `make check`、`make precheck` 和全部 OpenSpec 严格校验通过。真实模型实验模块仅作静态检查，实际模型回复及其影响仍未验证。

## 受测版本

| 步骤 | 本地验收提交 | 本地受测树 | 原补丁重算的完整树 |
| --- | --- | --- | --- |
| 20 | `4c76a79fabe1bb530f651f83547cd31ed27962b1` | `ed4ec70a429dc85b241c639274b5acbc7bce6b21` | `16f031efa01859ff6b9781c07610995acf9656a6` |
| 21 | `533e10b775e51223149da8f26bb220969663a1f0` | `155bdc5251689b0a0f51be753facff83c5dfbf5c` | `23804aa94d1ab516e03d291ffbe736ce42c48761` |

两份原始补丁的 SHA-256 均与清单匹配。第 20 步有 438 个跟踪文件、第 21 步有 449 个跟踪文件与对应清单树逐项一致。两步均有同一处差异：连接真实模型的 `apitest/security_experiments.py` 未写入受测检出。原文件在内存中通过 Python 语法解析，禁网容器中的 Gitleaks 对该文件扫描退出 0。将原文件的 blob 标识纳入树计算后，两次均得到清单目标树。默认门禁报告记录的受测提交是表中的本地验收提交。

## 第 20 步

| 执行命令 | 退出码 | 实际观测 |
| --- | ---: | --- |
| `python3 apitest/run_all.py --case api/audit-events/GET.py` | 0 | 学生请求得到 403，教师可按请求 ID 查到拒绝记录。跨班与不存在记录的响应一致。审计存储故障时写入返回 503，业务计数保持一致，健康探针返回 200。列表预热 5 次后测量 100 次，p95 为 5.44 ms |
| `python3 apitest/run_all.py --case api/teaching-audit/POST.py` | 0 | 正常请求的身份、技能、模型用量与持久审计一致。失败和取消都有记录，用量缺失有明确标记，审计结果中未出现正文或令牌测试标记 |
| `openspec validate --all --strict` | 0 | 17 项通过 |

## 第 21 步

第 21 步补丁的 19 个文件全部纳入验收检出。重新构建后，`/live`、`/ready` 在本机 18081 端口均返回 HTTP 200。

| 执行命令或检查 | 退出码 | 实际观测 |
| --- | ---: | --- |
| `COMPOSE_ENV_FILES=.env.test make check` | 0 | Go 检查和构建、前端构建、12 组前端测试、OpenAPI 一致性和 25 组接口回归全部通过。全量运行时审计列表 p95 为 4.82 ms |
| 全量检查内的 `make check-security` | 0 | 732 个可达历史 blob、工作树、运行中前端及测试日志扫描均为 0 命中。npm 漏洞计数为 0，两个 Go 模块的可达通告计数均为 0 |
| `make precheck` | 0 | SEC-01、02、03、04、07 全部为 GREEN |
| `openspec validate --all --strict` | 0 | 17 项通过 |
| `go test -count=1 ./internal/auditlog ./internal/auth ./internal/grading ./internal/securityaudit` | 0 | 禁网容器内单独重跑，四个包均通过 |
| 核对本步 15 个 Markdown 文件中的相对文件链接 | 0 | 38 个不同的“来源文件、链接路径”组合均指向存在的文件 |

**两项独立复现。** 对同一审计列表分别以匿名、测试学生、测试教师身份发起请求，返回码依次为 401、403、200，教师查询遵守每页两条的上限，探针退出 0。另行运行 `SECURITY_PROVE_RED=1 make check-security` 时，合成样本触发扫描器退出 1、make 退出 2，与正常门禁的绿色结果形成对照。

本次仅作本机验证。上述两份验收快照位于独立克隆；协作分支仍为第 19 步 `a708dbf`。第 20–21 步的原样提交与推送尚未完成。
