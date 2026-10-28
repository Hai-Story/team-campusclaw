# 能力与 Scenario 对应

运行底座能力：team-runtime-foundation。这里只建立可追溯关系；行为用例在代码阶段执行。

| Requirement（可观察能力） | Scenario |
| --- | --- |
| R1 可复现启动与配置校验 | S1完整配置启动；S2缺必填；S3非法端口 |
| R2 匿名健康与存活 | S4正常依赖；S5依赖不可达；S6依赖超时；S7非法请求/方法 |
| R3 请求标识与结构化日志 | S8一致追踪；S9不采信外部标识 |
| R4 默认拒绝与不存在 | S10无/失效/伪造会话；S11不存在路径 |
| R5 优雅停机 | S12在途请求与10秒退出 |

四类失败：参数非法=S3/S7；无权限=S10；资源不存在=S11；依赖不可达=S5/S6。每条Scenario均在change/specs中给出GIVEN/WHEN/THEN。

| 第7课材料能力 | 交付文件与检查 |
| --- | --- |
| K1 仓库与协作入口 | README、.gitignore、.env.example、docs/team.md、平台设置待办 |
| K2 规约四件套 | change内proposal/design/specs/tasks，严格校验 |
| K3 工程规则 | AGENTS与harness/rules八类文件，路径及检查方式核对 |
| K4 流程技能 | .agents/skills的6个工作流与7个项目技能，前置信息和步骤核对 |
| K5 目录/运行设计 | docs/architecture、5个模块README、配置字典与S1-S12规划 |
| K6 可核对证据 | checklist/verification/references/repository-settings |

K1-K6是第7课仓库材料的核对点，不另造虚拟应用需求。B1-B7为学期候选，讨论后各建业务change再形成Scenario和用例。
