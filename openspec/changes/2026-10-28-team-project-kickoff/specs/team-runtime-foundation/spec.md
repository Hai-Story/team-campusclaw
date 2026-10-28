## Purpose

为 CampusClaw 小组后续增量开发提供可重复启动和可观察的运行基线，明确配置失败、匿名探针、依赖故障、请求追踪及默认拒绝的对外行为，使不同成员能按同一套条件验证底座，而不依赖会话中的口头说明。本文件为后续实现契约，第7课仅提交规划。

## ADDED Requirements

### Requirement: R1 可复现启动与配置校验

系统 MUST 在配置齐全时使用 README 所列的一条启动命令拉起应用与依赖；docs/configuration.md 的必填变量缺失或取值非法时 MUST 在接受业务请求前非零退出，日志只显示缺失键名与原因，不得回显敏感值。

#### Scenario: S1 完整配置后启动
- **GIVEN** 全新 clone、Docker 可用、必填环境变量齐全且依赖可达
- **WHEN** 成员按 README 执行 docker compose up --build -d
- **THEN** 120 秒内 GET /health 返回 200 且 status=ok，成员无需口头补充步骤

#### Scenario: S2 必填配置缺失
- **GIVEN** 独立测试环境删除 SESSION_SECRET，其他配置有效
- **WHEN** 启动 API
- **THEN** API 以非零状态退出，日志包含 SESSION_SECRET 键名，不监听业务端口，不输出其他配置原值

#### Scenario: S3 参数非法
- **GIVEN** API_PORT=70000，其他必填配置有效
- **WHEN** 启动 API
- **THEN** API 非零退出，明确报告 API_PORT 超出 1 到 65535，不改用默认端口继续运行

### Requirement: R2 匿名健康与存活探针

系统 SHALL 提供免认证的 GET /health 与 GET /live。/health 仅在 storage、search、tools 均为 up 时返回 200 和 status=ok，否则返回 503 和 status=degraded；/live 不访问外部依赖，进程存活时返回 200 与 status=alive。两者均含 service=campusclaw、1 至 64 字符的 version、UTC RFC3339 time、request_id，/health 额外含 dependencies；不得暴露地址、口令、令牌、拓扑或堆栈。

#### Scenario: S4 匿名查看正常依赖
- **GIVEN** 三项依赖正常，调用者未登录
- **WHEN** GET /health，不传任何查询参数
- **THEN** 200，status=ok，dependencies 的三个值均为 up，响应及响应头含同一 request_id，字段中不含机密或连接地址

#### Scenario: S5 依赖不可达
- **GIVEN** 独立测试环境的 search 适配器连接被拒绝，其他依赖正常
- **WHEN** GET /health 后再 GET /live
- **THEN** /health 在 3 秒内返回 503、status=degraded、search=down；/live 返回 200、status=alive；响应不泄露内部地址

#### Scenario: S6 依赖超时
- **GIVEN** 三项测试依赖均接受连接但不回应
- **WHEN** GET /health
- **THEN** 单项探测在 2 秒超时，整个响应不超过 3 秒并返回 503，不调用模型或无限重试

#### Scenario: S7 探针请求非法
- **GIVEN** API 正常运行
- **WHEN** GET /health?detail=full 或 POST /health
- **THEN** 前者返回 400、后者返回 405，均不泄露配置或改变服务状态

### Requirement: R3 请求标识与结构化日志

系统 MUST 为每个请求生成 16 至 64 字符、仅含 ASCII 字母数字和连字符的 request_id，在 X-Request-ID 响应头、响应体适用字段与日志中保持一致；不采信外部传入的标识。每条请求完成日志 SHALL 为一行 JSON，含 ts、level、module、request_id、method、path、status、duration_ms，不记录查询参数、请求体、Cookie、Authorization 或敏感配置。

#### Scenario: S8 一次请求可追踪
- **GIVEN** API 正常运行
- **WHEN** 请求 GET /health
- **THEN** 返回的 X-Request-ID 与 JSON request_id、该请求完成日志一致，duration_ms 为非负数，日志每行可独立解析为 JSON

#### Scenario: S9 外部标识注入
- **GIVEN** 调用者传入自定 X-Request-ID=attacker-controlled
- **WHEN** 请求 GET /live
- **THEN** 返回服务端重新生成的合规标识，日志与响应一致，不使用 attacker-controlled

### Requirement: R4 业务入口默认拒绝与不存在处理

系统 MUST 显式登记匿名白名单；/health 与 /live 可匿名访问，其他业务入口默认要求服务端认证。无有效会话时 SHALL 返回 401，不返回业务数据、不产生写入；非 API 的不存在路径 SHALL 返回 404，不返回应用源码、文件目录或堆栈。业务资源跨班规则在迁移与后续业务规约中配对验证。

#### Scenario: S10 无权限访问
- **GIVEN** API 正常，无有效会话
- **WHEN** 不带令牌、使用失效令牌、使用伪造令牌分别访问 /api/me
- **THEN** 三次均返回 401，无用户或班级业务数据，不产生写入，日志不含令牌

#### Scenario: S11 资源不存在
- **GIVEN** API 正常
- **WHEN** 直接请求后端不存在的 /__missing_resource__
- **THEN** 返回 404，响应不含目录列表、文件内容或堆栈

### Requirement: R5 优雅停机

系统 SHALL 在收到正常停止信号后停止接收新业务请求，允许在途请求最多处理 10 秒后退出；不无限等待，不以业务请求成功掩盖已知未完成的工作。

#### Scenario: S12 停机与在途请求
- **GIVEN** 一个可控测试请求尚需 1 秒完成，服务正常运行
- **WHEN** 向 API 发送正常停止信号
- **THEN** 在途请求得到原定响应，新请求不再被接收，进程在 10 秒内退出
