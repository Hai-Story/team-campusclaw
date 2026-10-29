## Context

动因与范围见 proposal.md。平台已有能力与技术约定见 docs/platform-overview.md，运行契约见对应 spec。

## Goals / Non-Goals

**Goals:** 固定目录职责、规则与技能入口、配置清单和运行验收行为，让下一次执行只需选择 tasks.md 中的一项。

**Non-Goals:** 不在运行底座中加入七项业务扩展，各项扩展另立 change。

## Decisions

### D1. 在现有平台上按模块增量开发

整合与扩展采用经过评审的模块清单，保留适用的 LICENSE；排除 .env、数据卷、uploads、output 和旧测试报告。放弃直接复制整个工作目录，避免混入本机配置与运行数据。

### D2. 分层沿用技术栈，逐模块评审迁移

浏览器 → web/Nginx → Go 路由边界 → 业务服务 → 数据访问接口；模型、向量库与 MCP 在适配层。SQL 仅在数据访问层，新模块按此放置；现有 tutor、mcp 的既有持久化方式在迁移 change 中逐项记录，不声称已经符合目标分层。具体目录见 docs/architecture.md，不为统一目录名做一次性大重构。

### D3. 健康与存活分开

规划 GET /live 为不查依赖的存活探针，GET /health 为聚合就绪探针，公开字段仅含 status、service、version、time、dependencies 和 request_id；依赖对外叫 storage/search/tools，分别映射 MySQL、Qdrant、MCP，值只为 up/down。单依赖超时 2 秒，并发探测，总请求预算 3 秒，不调用模型或嵌入网关。模型配置完整性在启动校验。

现有 /health 只返回 status=ok，不查数据库。这里明确规划为新仓库契约的扩展，迁移时须补回归与调用方检查；不把旧接口声称为已通过第7课依赖检查。放弃单一存活探针兼任就绪，因为依赖断开时会产生误判。

### D4. 配置外置与兼容边界

.env.example 只留键名；变量含义与必填性见 docs/configuration.md。敏感项与依赖地址缺失立即失败。现有 API 的 /api 路径、HTTP 状态码、成功体与 error 字段沿用当前平台约定；未来业务码或 /api/v1/ 改造另提 change，保持现有接口兼容。运行底座新探针使用独立明确定义的结构。

### D5. 规则与技能入库，代码阶段再验证行为

AGENTS.md 指明开工前读取哪些 harness/rules 文件；五类必修规则各有独立文件，AI、数据、评审也各有一份。Codex 工作流由 openspec init --tools codex 生成，另外补充严格校验、按规约补测试与发布等项目技能。放弃只写 docs/规范汇总而不声明加载入口的做法。此轮只做静态与形式验证；新会话生成代码的实证和组员异机启动保留到代码阶段。

## Risks / Trade-offs

- [七项扩展依赖交错] → docs/project-brief.md 列明作业、题目、知识点与最终批改版本的依赖，不假定先完成某个模块。
- [合并保护未生效] → 按 docs/lesson-07/repository-settings.md 配置并验证主干保护。
- [验收状态失真] → 验证记录与实际结果对应，未执行的任务不勾选。
- [旧实现与新规则不一致] → docs/platform-overview.md 逐项列兼容处置，后续 change 决定迁移，不在第7课修改代码。

## Migration Plan

先确认主干保护，按评审后的模块清单整合现有能力，逐项实现并验证。发布前完成运行验收与回滚演练，通过后归档。
