# 个人基线与运行说明

## 来源

- 仓库：[MrKen20/CampusClaw](https://github.com/MrKen20/CampusClaw)
- 固定提交：[e5ffbb1677dfad4f5347a4023dc88a5b607c429a](https://github.com/MrKen20/CampusClaw/commit/e5ffbb1677dfad4f5347a4023dc88a5b607c429a)
- 本轮核对方式：读取个人任务聊天、该提交对应的本地源码、README、配置样例、路由入口与模块目录；未重新启动系统或重复模型调用。

源码显示既有登录与班级隔离、资料管理、混合检索、教学智能体与 SSE 会话、技能、MCP 管理和 Swagger。个人项目的历史验收结果属于个人基线，不计为小组仓库本课运行验收。

## 体验个人基线

下列命令在单独的个人基线目录执行。需要 Git、Docker Desktop 与课程提供的网关配置；真实口令、网关密钥各人在本机填写。

```bash
git clone https://github.com/MrKen20/CampusClaw.git campusclaw-personal-baseline
cd campusclaw-personal-baseline
git checkout --detach e5ffbb1677dfad4f5347a4023dc88a5b607c429a
cp .env.example .env
# 在本机按该提交的 .env.example 说明填写配置
docker compose up --build -d
```

默认浏览入口 http://localhost:8080，Swagger 为 /swagger/，端口可在该项目 WEB_PORT 调整。停止服务使用 `docker compose down`，保留数据卷；涉及清空数据的操作须单独明确授权。

本轮仅检查这些步骤与源码/配置的一致性；成员启动后自行记录环境和实际结果。后续小组运行步骤将在源码迁移及骨架验证完成后写回本仓库 README。

## 与课件样例及目标规则的差异

| 已核对的个人事实 | 小组承接方式 |
| --- | --- |
| API 使用 /api/，成功体是资源结构，失败为 error 字段，无独立业务码 | 按真实 OpenAPI 的 HTTP 信号判定；错误码封装或 /api/v1/ 另提兼容性 change |
| 页面在 App.tsx 通过 page 状态切换，不是 /console 等独立 URL | 页面清单记录真实状态键；引入 URL 路由另评审，不照抄课件页面地址 |
| 不透明 Bearer Token 对应服务端会话；前端存 localStorage | 保留为明确的迁移例外 E1；新认证方案须先立规约、迁移与注销回归，不能声称已改为内存或 Cookie |
| /health 只返回 status=ok，作为存活检查 | 新运行底座规划 /health 为就绪、/live 为存活；需补兼容回归后再验收 |
| 部分业务包含持久化/迁移代码 | 新增模块遵守数据访问边界；旧包整改按后续 change 分批做，不能仅凭目录名声称无反向依赖 |
| 配置含已有非敏感默认值，检索阈值键名为 VECTOR_SCORE_THRESHOLD | 本课只提取键名；运行迁移按新配置契约校验，不改写成课件 SEARCH_VECTOR_MIN |
| API 用例为 apitest/cases/.../*.py | 继承真实格式；不凭示范 .json 改写全部测试 |
| 运行门禁为若干现有命令，仓库无 make check | 文档检查现在可运行；make check 只作为后续统一运行门禁任务，不宣称现已可用 |

E1 的理由是本课固定来源、保持文档交付边界；失效条件为后续认证迁移 change 验收通过。该例外不允许新增模块继续扩大令牌持久化范围。

## 工具与后续迁移

OpenSpec 1.13.0 可用 `npm install -g @fission-ai/openspec@1.13.0` 安装（需兼容 Node.js）；版本通过 `openspec --version` 核对。初始化已执行 `openspec init --tools codex --language zh-CN`，成员 clone 后复用已入库技能。升级工具作为单独维护动作评审。

后续迁移仅从固定提交提取清单中的源码、锁文件和许可；排除 .env、uploads、output、数据卷和历史报告。先检查密钥与许可证、评审迁移 diff，再运行匹配场景的测试。个人工作目录的未提交 README 修改保持原状。
