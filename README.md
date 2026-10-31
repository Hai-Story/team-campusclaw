# CampusClaw 中小学教研智能体功能扩展

面向中小学教师与学生，在现有平台上增量开发，扩展备课、命题、学习诊断与课后巩固能力，以本班资料和可核对的证据支撑教学。

[小组仓库](https://github.com/Hai-Story/team-campusclaw) · [选题与边界](docs/project-brief.md)

## 现有平台能力

平台已具备服务端会话登录、角色与班级隔离、资料上传与管理、可追溯知识库检索、提示词与 AI 技能配置、SSE 流式学习会话，以及 MCP 工具接入。小组在这些能力上围绕教学场景进行增量开发。

## 第7课交付

项目定位、工程规范、Harness 规则与技能、OpenSpec 四件套、配置键名和目录职责已经确定，运行底座按任务清单逐项实施和验证。

规划七项扩展：智能备课、智能命题、学情诊断、知识图谱、间隔复习、讲题模拟、学生互评。首轮业务模块与优先级待确定。

## 获取与检查第7课材料

需要 Git；规约检查使用 OpenSpec CLI 1.13.0。已有 CLI 时：

```bash
git clone https://github.com/Hai-Story/team-campusclaw.git
cd team-campusclaw
openspec validate 2026-10-28-team-project-kickoff --strict
git diff --check
```

现有能力、技术结构与工具准备见 [现有平台概况](docs/platform-overview.md)。运行验收以 [运行底座规约](openspec/changes/2026-10-28-team-project-kickoff/specs/team-runtime-foundation/spec.md) 为准。

## 用户与场景

| 用户 | 教学任务 | 能力与页面规划 |
| --- | --- | --- |
| 教师 | 整理本班资料、配置智能体与技能 | 复用现有资料、智能体、技能与 MCP 工作区 |
| 教师 | 备课、命题、诊断与组织互评 | 七项扩展按后续 change 逐项交付 |
| 学生 | 本班资料检索、智能体学习、回看出处 | 复用资料库、知识问答、会话能力 |
| 学生 | 讲题、复习、练习与互评 | 依据各模块规约校验身份、流程和结果 |

## 小组分工

| 姓名 | 分工 | 主要职责 |
| --- | --- | --- |
| 曾钰杰 | 制定规约 | 明确需求边界、失败处理与验收条件，维护 OpenSpec 需求和设计。 |
| 郑靖炫 | 开发系统 | 按规约完成前后端、智能体与工具接入，保持主路径可运行。 |
| 张海峰 | 项目验收 | 核对功能、失败路径与权限，组织接口和页面验证，整理测试及演示材料。 |

## 开发流程

- 主干为 `main`，合并保护见[仓库设置](docs/lesson-07/repository-settings.md)。
- 变更名采用 `YYYY-MM-DD-kebab-case`，开发分支默认使用 `codex/<change>`。
- 按 Propose → Review → Apply → Verify → Archive 推进；一次实现一个 task，运行 verify 后勾选并提交。
- 提交信息包含 `Change:`、`Verify:`、`Evidence:`；合并前完成非作者评审。
- 按分工认领变更、提交成果并参与交叉评审，保留各自的协作记录。

## Harness 入口

Codex 从 [AGENTS.md](AGENTS.md) 读取开工规则，再按其中的加载矩阵读取 `harness/rules/`。流程技能在 `.agents/skills/`，包含 OpenSpec 工作流和项目技能。规划使用 `$openspec-propose`，校验使用 `$campusclaw-validate-change`，实现和归档分别使用 `$openspec-apply-change`、`$openspec-archive-change`。

## 导航与许可

- [功能扩展规划](docs/project-brief.md)
- [目录、分层与未来运行骨架](docs/architecture.md)
- [配置字典与密钥轮换](docs/configuration.md)
- [第7课提交证据](docs/lesson-07/verification.md)

沿用仓库现有 [MIT License](LICENSE)。复用与扩展代码时保留适用的版权和许可。
