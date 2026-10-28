# Harness 使用与检查

本仓库以 Codex 为主要 Harness。根 AGENTS.md 是加载入口，明确要求读取 harness/rules 中适用文件；规则正文全部入库，不依赖临时会话。自定义流程技能在 .agents/skills，开发技能与产品里的教学技能分开。

| 课程写法 | 本仓库的 Codex 入口 | 底层动作 |
| --- | --- | --- |
| /opsx-propose | $openspec-propose | 先读上下文，再创建四件套，只规划 |
| /opsx-validate | $campusclaw-validate-change | openspec validate <change> --strict，加语义与映射核对 |
| /opsx-apply | $openspec-apply-change | 一次一个已授权task，运行verify再勾选 |
| /opsx-archive | $openspec-archive-change | 必需任务验收后归档，同步长期spec |

课程命令是不同Harness的入口表示，本机Codex初始化使用技能，不声称已经安装同名斜杠命令。OpenSpec 1.13.0 自动生成6个工作流技能（propose、apply、archive、explore、sync、update）；另有7个项目技能：校验规约、补测试、加接口、评审、发布、数据变更、提示词评测。

## 实例入口

文档检查可在新Codex会话输入：

> 只读取当前小组仓库，使用 $campusclaw-validate-change 检查 2026-10-28-team-project-kickoff。列出加载的规则、能力与Scenario映射及未执行事项，不编写代码、不启动服务。

进入后续代码阶段需先由小组评审change并明确执行任务；示例：

> 使用 $openspec-apply-change，只完成 2026-10-28-team-project-kickoff 的一个已获准task；按该条verify验证后停止。

## 生效验证

第7课静态核查AGENTS入口、规则路径、SKILL.md前置信息及内部引用。静态检查不能证明另一个会话会正确生成代码。

后续行为核查在隔离分支做两次：第一次让Harness只加一个获准接口/页面，检查分层、命名、错误与四状态；第二次在无历史上下文的会话重复，比较规则一致性。另做一次故意违规的隔离修改（如在handler写SQL或去掉归属判定），审查应指出具体规则并使相关测试变红。完成后恢复修改、记录会话/提交/命令与结果，不改业务范围。

## 官方依据

[AGENTS.md 发现机制](https://learn.chatgpt.com/docs/agent-configuration/agents-md)说明仓库指令加载；[技能目录与SKILL.md](https://learn.chatgpt.com/docs/build-skills)说明 .agents/skills 的仓库级发现。外部工具的版本与能力以实际输出为准，文件存在不等于行为验收通过。
