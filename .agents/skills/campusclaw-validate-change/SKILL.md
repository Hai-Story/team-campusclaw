---
name: campusclaw-validate-change
description: 严格校验 CampusClaw 的 OpenSpec change、能力与场景映射及范围；用于实现前或规约修订后的检查，不执行代码任务。
---

# campusclaw-validate-change

输入：仓库路径、change 名称。

先读根 AGENTS.md、对应 proposal/design/specs/tasks 和 docs/lesson-07/traceability.md（若该 change 适用）。运行 `openspec context --json` 确认根，严格校验使用 `openspec validate <change> --strict`；记录实际退出码，不把课程 /opsx-validate 当作已安装命令。

逐条核对 Requirement 与 Scenario：前置条件、操作、可观测结果齐全；参数非法、无权限、资源不存在、依赖不可达四类负向条件有具体结果；每项任务有 verify，骨架在先。检查 design 的取舍与未决事项是否影响本轮契约。影响范围的歧义先补 change，不自行定功能优先级。

产出：检查命令/退出码、两列表映射、明确缺口与就绪结论。形式通过只表示规约可审阅，不表示实现、组员评审或运行通过。除非用户同时授权修订，否则只报告问题，不进入 Apply。
