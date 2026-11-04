# CampusClaw 中小学教研智能体

面向教师与学生，在现有平台上增量开发，提供本班资料检索、学习会话与客观题异步批改。

[小组仓库](https://github.com/Hai-Story/team-campusclaw) · [扩展规划](docs/project-brief.md)

## 功能

- 服务端会话登录、教师/学生角色与班级隔离，资料上传、编辑、下载和删除。
- 关键字、向量与混合检索，带资料出处的问答、SSE学习会话、教学技能与MCP工具接入。
- 教师发布客观题，学生提交后立即获得任务编号；后台批改，页面轮询并在刷新后恢复查询。
- 提交/任务/发件箱同事务受理；RabbitMQ持久消息、独立消费者、租约接管、结果去重、有界重试与死信。

评分按去除首尾空白后的答案与标准答案精确匹配，得到满分或0分；模型只生成反馈。七项教学扩展的候选范围见扩展规划，按独立规约推进。

## 启动隔离环境

需要Docker Compose、Python 3、Node.js 24及npm；规约检查使用OpenSpec CLI 1.13.0。以下配置使用仓库内的本地模型网关替身，不调用付费模型。

```bash
git clone https://github.com/Hai-Story/team-campusclaw.git
cd team-campusclaw
python3 scripts/dev_env.py
docker compose --profile test up --build -d
npm --prefix frontend ci
```

打开 <http://127.0.0.1:18080>。账号：教师`teacher_a`、学生`student_a1`/`student_a2`（班级A），学生`student_b1`（班级B）。教师和学生口令分别读取本机`.env`中的`SEED_TEACHER_PASSWORD`、`SEED_STUDENT_PASSWORD`；不要提交或公开口令。

脚本只在文件不存在时生成`.env`和`.secrets/mysql-root`，已有配置原样保留；保留与数据卷配套的凭据。只有web映射到本机端口，数据库、队列和模型替身位于内部网络。替换为获准模型服务时按[配置字典](docs/configuration.md)填写模型地址、名称和凭据，移除test profile；接口行为验证与真实模型质量评测分开。

演示：教师进入“作业与批改”发布题目、标准答案及满分；退出后以学生登录并提交答案，查看等待、批改中、完成或失败状态。学生题目页不显示标准答案。

## 检查

在上述环境就绪、前端依赖已安装后执行：

```bash
make check
make precheck
make check-security
```

`make check`依次运行Go检查、前端构建及行为测试、OpenSpec、接口文档一致性、异步回归和资产门禁。接口用例会停启隔离消费者/队列并注入数据库约束故障，仅允许`campusclaw-team-async`与本地网关配置运行；请在专用验证环境执行。

当前功能回归通过；资产门禁因依赖风险返回非零，`make precheck`因教学技能审计字段缺口返回非零。实际红项与处置范围见[安全自查](reports/security-precheck.md)和[资产基线](reports/security-baseline.md)，不能把登记风险当作检查通过。

- [异步响应与页面验证](reports/async-response.md)
- [权限](reports/risk-authz.md)、[并发](reports/risk-concurrency.md)、[事务](reports/risk-transaction.md)复现
- [评审状态](reports/review-01.md)：非作者独立复现、异机启动与主干保护待完成；之后再合入、归档和标记版本。

运行接口文档：<http://127.0.0.1:18080/swagger/index.html>，OpenAPI原文在同目录`openapi.json`。`/live`检查进程，`/health`聚合数据库、向量库及MCP服务就绪，不发起计费生成。

## 小组分工

| 姓名 | 分工 | 主要职责 |
| --- | --- | --- |
| 曾钰杰 | 制定规约 | 明确需求边界、失败处理与验收条件，维护 OpenSpec 需求和设计。 |
| 郑靖炫 | 开发系统 | 按规约完成前后端、智能体与工具接入，保持主路径可运行。 |
| 张海峰 | 项目验收 | 核对功能、失败路径与权限，组织接口和页面验证，整理测试及演示材料。 |

## 开发流程

从[AGENTS.md](AGENTS.md)加载规则，按OpenSpec的Propose → Review → Apply → Verify → Archive推进。一次实现一个task，实际运行verify后勾选并提交；提交说明保留必要的Change、Verify、Evidence。开发分支使用`codex/<change>`，通过PR进行非作者评审，不代他人署名。

- [架构与目录](docs/architecture.md) · [配置](docs/configuration.md) · [数据字典](docs/data-dictionary.md)
- [异步批改规约](openspec/changes/2026-11-04-async-grading/specs/async-homework-grading/spec.md)
- [仓库合并设置](docs/lesson-07/repository-settings.md)

沿用[MIT License](LICENSE)，保留适用的版权和许可。
