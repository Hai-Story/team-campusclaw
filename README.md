# CampusClaw 中小学教研智能体功能扩展

面向中小学教师与学生，在个人 CampusClaw 基础上规划备课、命题、学习诊断与课后巩固能力，以本班资料和可核对的证据支撑教学。

[小组仓库](https://github.com/Hai-Story/team-campusclaw) · [个人项目](https://github.com/MrKen20/CampusClaw) · [选题与边界](docs/project-brief.md) · [第 7 课核查表](docs/lesson-07/checklist.md)

## 本次交付

2026-10-28：提交项目定位、协作规范、Harness 规则与技能、OpenSpec 四件套、配置键名和目录职责说明。运行底座的契约与后续任务已经写入规约，运行验收证据按任务实际执行后补齐。

七项扩展沿用 2026-10-14《CampusClaw 小组项目选题说明》：智能备课、智能命题、学情诊断、知识图谱、间隔复习、讲题模拟、学生互评。首轮业务模块与优先级待小组确定；本课范围是项目启动材料。

## 获取与检查本课材料

需要 Git；规约检查使用 OpenSpec CLI 1.13.0。已有 CLI 时：

```bash
git clone https://github.com/Hai-Story/team-campusclaw.git
cd team-campusclaw
openspec validate 2026-10-28-team-project-kickoff --strict
git diff --check
```

CLI 安装与个人项目体验步骤见 [个人基线与运行说明](docs/baseline.md)。该页固定了个人仓库提交号，并明确运行命令所对应的仓库。本课默认分支提交的是规划材料，后续从本仓库一条命令运行的验收以 [运行底座规约](openspec/changes/2026-10-28-team-project-kickoff/specs/team-runtime-foundation/spec.md) 为准。

## 用户与场景

| 用户 | 教学任务 | 能力与页面规划 |
| --- | --- | --- |
| 教师 | 整理本班资料、配置智能体与技能 | 复用个人项目资料、智能体、技能、MCP 工作区 |
| 教师 | 备课、命题、诊断与组织互评 | 七项扩展按后续 change 逐项交付 |
| 学生 | 本班资料检索、智能体学习、回看出处 | 复用资料库、知识问答、会话能力 |
| 学生 | 讲题、复习、练习与互评 | 依据各模块规约校验身份、流程和结果 |

## 小组协作

| 成员 | 分工 | 交付责任 |
| --- | --- | --- |
| 曾钰杰 | 制定规约 | 用户与边界、需求与设计、场景、任务、证据要求 |
| 郑靖炫 | 开发系统 | 按规约完成前后端及智能体、工具接入，逐项验证 |
| 张海峰 | 项目验收 | 核对主路径、失败路径和权限，组织测试与演示 |

GitHub 账号对应、学号及成员入库状态见 [团队约定](docs/team.md)，由本人补齐。三人共同评审规约、规则与架构。

- 主干：`main`。业务编码前由仓库所有者落实 [成员与主干保护](docs/lesson-07/repository-settings.md)。
- 变更名：`YYYY-MM-DD-kebab-case`；开发分支默认 `codex/<change>`，一条分支只承接一个 change。
- 流程：Propose → Review → Apply → Verify → Archive；一次实现一个 task，运行其 verify 后勾选并提交。
- 提交信息写清改动与规约依据，正文带 `Change:`、`Verify:`、`Evidence:`；PR 用仓库模板。
- 成员 clone 后先核对文档；代码阶段由另一成员按 README 从零启动并记录真实结果。

## Harness 入口

Codex 从 [AGENTS.md](AGENTS.md) 读取开工规则，再按其中的加载矩阵读取 `harness/rules/`。流程技能在 `.agents/skills/`，包含 OpenSpec 官方生成的工作流和项目自定义技能。详见 [Harness 使用与检查](docs/harness.md)。

## 导航与许可

- [项目说明与课程要求映射](docs/project-brief.md)
- [目录、分层与未来运行骨架](docs/architecture.md)
- [配置字典与密钥轮换](docs/configuration.md)
- [本课提交证据](docs/lesson-07/verification.md)
- [能力与场景对应表](docs/lesson-07/traceability.md)
- [课程参考页与 15 个子页面核对](docs/lesson-07/references.md)

沿用仓库现有 [MIT License](LICENSE)。个人代码迁移时保留来源、版权与许可，并单列迁移清单。
