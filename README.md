# CampusClaw 中小学教研智能体

面向教师与学生，在现有平台上增量开发，提供本班资料检索、智能备课、学习会话、作业练习与教师终评。

[小组仓库](https://github.com/Hai-Story/team-campusclaw) · [扩展规划](docs/project-brief.md)

## 功能

- 服务端会话登录、教师/学生角色与班级隔离，资料上传、编辑、下载和删除。
- 关键字、向量与混合检索，带资料出处的问答、SSE学习会话、教学技能与MCP工具接入。
- 九学科知识点目录，教师维护章节并关联本班资料与同学科作业，学生收藏及取消收藏。
- 教师指定本班讲义生成教案，包含目标、重点难点、活动、时间分配和出处；私有草稿可编辑、保存与重新打开。
- 学生围绕单题开展讲题模拟，按审题、选方法、推导、检查保存讨论、小结和改进建议，停止或刷新后可恢复。
- 两端查看知识图谱及关联资料、练习和复习入口；教师维护关系并审核讲义提取候选，薄弱点按最新作答的教师终评计算。
- 教师按知识点、题型、难度和题量生成命题草稿，修改审核后入私有题库；支持选题、排序、配分、保存试卷草稿并发布为班级多题作业。
- 教师按语文、数学、英语、物理、化学、生物、政治、历史、地理发布作业并审阅作答；学生提交后查看AI初判、教师终评和分学科错题，可逐题生成带本班资料出处的错题讲解。
- 学生左侧“提交作业”显示新作业未读数量圆标；在该页面查看单题或多题提醒、标记已读，前台每30秒刷新，已读状态跨登录保留。
- 学生从错题和收藏知识点整理复习卡，按九学科查看到期任务、反馈熟悉程度、回看完成历史并调整间隔；站内提示有效待办。
- 教师确认错因标签，两端按学科查看终评得分率、正确率、知识点及趋势，回查具体作答；支持基于真实统计的诊断解释和经批准的本班学习工具。
- 提交/任务/发件箱同事务受理；RabbitMQ持久消息、独立消费者、租约接管、结果去重、有界重试与死信。

AI仅给出对错初判与反馈，教师独立确认最终对错及分数。错题以每份作业的最新作答为准，教师终评优先；改判正确即移出错题。教学扩展按独立规约分批推进，范围见扩展规划。

## 启动

需要Docker Compose、Python 3、Node.js 24及npm；规约检查使用OpenSpec CLI 1.13.0。作业初判使用部署者配置的模型服务。

```bash
git clone https://github.com/Hai-Story/team-campusclaw.git
cd team-campusclaw
python3 scripts/dev_env.py
# 在本机.env填写CHAT_BASE_URL、CHAT_API_KEY、CHAT_MODEL后启动
docker compose --profile test up --build -d
npm --prefix frontend ci
```

打开 <http://127.0.0.1:18080>。账号：教师`teacher_a`、学生`student_a1`/`student_a2`（班级A），学生`student_b1`（班级B）。教师和学生口令分别读取本机`.env`中的`SEED_TEACHER_PASSWORD`、`SEED_STUDENT_PASSWORD`；不要提交或公开口令。

脚本只在文件不存在时生成`.env`和`.secrets/mysql-root`，已有配置原样保留。保留与数据卷配套的凭据；模型密钥仅在服务端。初判预算默认60秒，租约70秒；检索的嵌入模型单独配置，参数见[配置字典](docs/configuration.md)。

演示：教师在“作业发布与批改”选择学科，点击“发布作业”填写题目、参考答案及满分；选中作业可查看详情，点击“批改作业”查看学生提交并确认分数和对错。学生在“提交作业”按学科作答，在“我的错题”查看有效错题并点击“错题讲解”查看学习提示与真实资料出处，依据不足时可联系教师补充资料；刷新可恢复本人记录，学生题目页不显示参考答案。

发布页的“从题库中添加”可选一道或多道同学科题目，调整顺序、标题与分值，核对后确认发布；发布后在“多题作业”中查看。取消选题会保留原发布表单。

九科学科名旁显示图标，班级资料另保留“其他”。初中原创示例覆盖54个主题，可供教师编辑并用于练习：

```bash
docker compose exec -T api /app/seed-junior -apply -teacher teacher_a
```

导入108个知识点、54条图谱关系、216道题和54份私有教案及其讲义；重复执行跳过已完成主题，不覆盖已有编辑。导入前备份数据库与uploads，索引失败后可重试。

完成主题导入后，可从已有题库补充27份示例作业（每科2份单题和1份四题作业），重复执行不覆盖编辑或成绩：

```bash
docker compose exec -T api /app/seed-junior -apply -homework -teacher teacher_a
```

教师在“知识点”维护章节与资源关联，学生可收藏知识点。在“智能备课”填写章节、目标和课时，选择本班讲义后生成；无检索依据时提示补充资料。教案仅创建教师可见，编辑后点击保存，出处可打开原资料。

学生在“讲题模拟”按学科选择作业，表达思路后进入下一阶段，检查阶段可完成讲题。过程独立保存，正式作答仍从“提交作业”提交。

在“知识图谱”完整视图中点选节点进入详情，点击“返回知识图谱”恢复原筛选；教师可维护先修、包含、关联关系，从资料提取候选，修改保存后确认审核入图。未审核候选仅创建教师可见，AI初判不计入薄弱样本。

页面发现新版本时会提示保存编辑后刷新；入口与版本信息不缓存，带指纹的静态资源独立缓存。

教师在“智能命题与题库”选择知识点和讲义，设置允许题型、难度及题量，生成后逐题核对、保存修改并确认审核入库。“新建试卷”可从题库选题，调整顺序和分值；已保存题面保持快照，题库改题后可明确选择更新。私有题目、答案、解析及试卷草稿仅创建教师可见。已保存试卷可发布到本班，发布后题面固定；学生在作业入口切换“多题作业”整份提交，教师逐题终评后形成正式总分。知识点、错题、图谱、诊断和复习中的题目可跳回相应试卷。

学生在“间隔复习”完成到期卡片，选择“已掌握”或“还需巩固”；“完成记录”保留反馈，“复习间隔设置”只影响后续安排。教师改判、重新作答或取消收藏后，失效卡片从待办移出。经批准的review_plan工具只读本人计划。

在“学情诊断”选择时间范围；教师可看本班或指定学生，学生只看本人。正式统计只读取最新作答的教师终评，样本不足显示为空；点开记录可回查题目和评语。“MCP 工具”中启用本班学习工具、连接并批准后，可绑定到教学技能用于会话查询。

## 操作审计

教师在“审计记录”按操作人ID、技能标识、结果和时间回查本班操作；学生无法查询。缺省显示最近24小时，单次最多31天，详情保留请求标识及实际用量。无模型调用、用量未上报和操作未完成分别显示，不把它们当作零消耗或成功。

审计库不可用时业务请求返回503；恢复后重试。记录只含身份、路由、对象和状态元数据，不含资料、作答或提示词正文。只读查询接口为`GET /api/audit-events`及`GET /api/audit-events/{id}`，筛选字段见OpenAPI。

## 检查

自动回归使用独立的18081与测试数据卷。前端依赖安装后执行：

```bash
python3 scripts/dev_env.py --test
docker compose --env-file .env.test --profile test up --build -d
COMPOSE_ENV_FILES=.env.test make check
make precheck
COMPOSE_ENV_FILES=.env.test make check-security
```

`make check`依次运行Go检查、前端构建及行为测试、OpenSpec、接口文档一致性、异步回归和资产门禁。接口用例仅接受`.env.test`中的`campusclaw-team-test`、18081及本地网关，停启与故障注入不影响18080的运行数据。

注入反例随`make check`运行。`SECURITY_PROVE_RED=1 make check-security`仅运行隔离的合成扫描反例，预期非零；正常入口仍扫描完整资产，红绿结果见[门禁证据](reports/gate-red-green.md)。模型安全实验显式执行`python3 apitest/security_experiments.py --live`，使用隔离数据库和真实模型，结果见[实验记录](reports/sec08-experiment.md)。审计回查与安全修复见[回查验证](reports/audit-recheck.md)、[安全检查](reports/security-fix-log.md)。

若本机 npm 镜像站审计请求失败，可仅在检查命令前加 `npm_config_registry=https://registry.npmjs.org`，不修改全局配置。

真实模型评测显式运行`python3 apitest/evaluate_model.py --live`，会调用本机配置的模型处理20条固定合成题；普通自动回归使用替身。

备课评测使用`python3 apitest/evaluate_lessons.py --live`，在隔离环境处理20条固定合成场景。教学扩展的本地验证见[教学扩展验证](reports/learning-expansion.md)。

讲题、图谱、命题评测分别使用`python3 apitest/evaluate_extensions.py --live --feature walk`、`--feature graph`、`--feature bank`；各运行20条固定场景，使用隔离数据和本机配置的真实模型。错题讲解评测使用`python3 apitest/evaluate_guidance.py --live`；诊断解释和学习工具评测分别使用`python3 apitest/evaluate_diagnosis.py --live`、`python3 apitest/evaluate_learning_tools.py --live`；复习工具使用同一脚本追加`--review`。

本地接口回归与安全门禁通过，依赖及教学技能审计已修复。九科工具真实模型补测各20/20场景通过；验证范围见[功能记录](reports/learning-expansion.md)、[安全自查](reports/security-precheck.md)和[资产基线](reports/security-baseline.md)。

- [异步响应与页面验证](reports/async-response.md)
- [权限](reports/risk-authz.md)、[并发](reports/risk-concurrency.md)、[事务](reports/risk-transaction.md)复现
- [评审状态](reports/review-01.md)：已追加张海峰独立复测；非作者人工评审、异机启动与主干保护待完成，之后再合入、归档和标记版本。

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
