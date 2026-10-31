# CampusClaw 小组仓库规则

项目：中小学教研智能体功能扩展。开工先读 README.md、对应 change 的 proposal/design/specs/tasks，复述其范围与 Non-goals，再按下表读取规则正文。验证结果记录在 docs/lesson-07/verification.md。

## 规则加载入口

| 工作范围 | 必须读取的规则 |
| --- | --- |
| 所有任务 | harness/rules/03-security.md、harness/rules/04-testing.md、harness/rules/08-review.md |
| 产品、页面、交互、视觉 | harness/rules/01-product.md |
| backend/、接口、配置 | harness/rules/02-backend.md |
| deploy/、运行、发布、日志 | harness/rules/05-ops.md |
| 模型、检索、提示词、技能、MCP | harness/rules/06-ai.md |
| 存储、迁移、删除、统计 | harness/rules/07-data.md |

这些文件为约束正文，不能只读取索引后宣称已加载。Codex 流程技能位于 .agents/skills/，选用相关动作的 SKILL.md；不把本项目的学习技能与开发技能混用。

规则分层为：本文件的常驻底线 → 领域规则 → change 的具体场景 → task 的 verify。实现前将适用规则落实到 Requirement/Scenario 和任务；阈值、失败结果或例外不明确时先补规约，不凭实现猜测，也不以局部约定放宽安全底线。

## 无规约不写代码

- 功能改动前必须有一个 `openspec/changes/YYYY-MM-DD-kebab-case/`，proposal.md、design.md、specs/<capability>/spec.md、tasks.md 齐全并严格校验通过。
- 歧义先改 change 中的规约，再改代码；以 Scenario 原文判定偏差，不由实现补充范围。
- Propose → Review → Apply → Verify → Archive；Apply 一次一个 task：实现、审查 diff、实际运行该条 verify、勾选、提交。未经运行不勾选。
- Apply 只修改 change 里的 delta；`openspec/specs/` 只由归档同步。运行任务未通过不得归档或打 skeleton 标签。
- 实现范围以经过评审的 change 与 task 为准，禁止扩展到 Non-goals。

## 三条常驻安全约束

1. 跨班单资源读取与不存在同形 404。比较时排除每请求不同的 request_id/time 等追踪字段，业务状态、结构、文案及数据须一致，不得泄露资源存在性。
2. 身份、角色与 class_id 每次从服务端会话和数据库读取；不采信客户端权限字段，前端隐藏入口不能代替授权；文件读取绑定已授权业务对象，uploads 不公开。
3. 口令只存 bcrypt 哈希；密钥只来自环境变量，缺失即启动失败；.env 不入库、不进镜像、前端、日志或报告，泄露先吊销/轮换。

## 检查与完成定义

文档阶段执行 `openspec validate 2026-10-28-team-project-kickoff --strict`、`git diff --check`，核对加载矩阵目标、技能的 name/description、文档链接及能力与场景对应关系。根 AGENTS.md 使用普通 Markdown；技能前置信息按 SKILL.md 格式校验。文档检查通过只表示规划可审阅。

代码阶段门禁按相关 change 的任务实施；统一 make check 建成前不得声明它已经通过。变更完成须满足场景、测试、门禁、文档同步、证据归档与非作者评审；涉及提示词须有评测，数据迁移须有回滚验证。不得放宽检查、删断言或编造测试结果让门禁变绿。

## 提交与评审

main 为共同基线。业务编码前配置主干保护，随后由 PR 合并、至少一名非作者评审；一个实现提交只完成一个 task。提交说明带 Change、Verify、Evidence 字段，值可核对。评审请求在一个工作日内响应。

规则/规约/门禁改动须写明原因与验证，由人审阅。例外写入对应规则，注明理由、范围、失效条件。

## 版本库内容

按独立变更分次提交，提交信息简短说明改动及必要验证。源代码、文本规约和必要配置入库，PDF、字体等二进制大文件及生成产物不入库；提交前检查文件类型和体积，忽略规则不能清除已入库的文件。

现有平台的兼容事项见 docs/platform-overview.md；已有功能说明不自动成为新增契约已验收的证明。证据文件只记录真实验证结果。
