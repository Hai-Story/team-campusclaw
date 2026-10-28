# 配置字典与机密边界

本仓库 .env.example 每行仅有 `键名=`。它由已核对个人基线的键名加运行底座规划键组成；本课值全部留空。下表是后续实现契约，运行前按届时 README 填写。

| 键 | 含义与后续校验 |
| --- | --- |
| APP_VERSION | 运行版本，必填 1 至 64 字符，随发布确定 |
| API_PORT / WEB_PORT / MYSQL_PORT | API、对外入口、数据库端口；必填整数 1 至 65535 |
| MYSQL_HOST / MYSQL_DATABASE / MYSQL_USER / MYSQL_PASSWORD | 数据库连接；全必填，密码保密，不记录连接串 |
| SESSION_SECRET | 会话密钥，必填至少 32 字节，仅服务端 |
| SEED_TEACHER_PASSWORD / SEED_STUDENT_PASSWORD | 演示教师/学生口令，必填，最终只保存 bcrypt 哈希 |
| UPLOAD_DIR | 服务端上传路径，必填，不能作为公开静态目录 |
| EMBEDDING_BASE_URL / EMBEDDING_API_KEY / EMBEDDING_MODEL / EMBEDDING_DIM | 嵌入网关；必填，维度正整数，地址由部署者批准 |
| CHAT_BASE_URL / CHAT_API_KEY / CHAT_MODEL | 模型网关；必填，密钥仅服务端，不由请求体覆盖 |
| QDRANT_URL | 向量依赖地址，必填，内部网络使用 |
| MCP_AUTH_TOKEN / MCP_ENCRYPTION_KEY / MCP_ALLOWED_BASE_URLS | 工具认证、加密和精确出站白名单，必填；加密键为 64 位十六进制 |
| MAX_UPLOAD_BYTES | 单文件上限，必填正整数；基线参考值 1048576 |
| SESSION_TTL_HOURS | 会话有效小时数，必填正数；基线参考值 24 |
| LOGIN_FAIL_LIMIT / LOGIN_FAIL_WINDOW_SECONDS | 登录限流阈值/窗口，必填正整数；基线参考 5 / 300 |
| CHUNK_AUTO_MAX_CHARS / CHUNK_AUTO_OVERLAP | 切片长度/重叠；前者正整数，后者非负且小于长度；基线参考 800 / 80 |
| VECTOR_SCORE_THRESHOLD / RRF_K / ASK_TOP_K | 检索阈值 0 到 1，融合 k 与命中数正整数；基线参考 0.25 / 60 / 4 |
| MCP_MAX_ROUNDS / MCP_MAX_CALLS_PER_TURN / MCP_MAX_CALLS_PER_CONVERSATION | 工具轮数/单轮与会话调用预算，必填正整数；基线参考 3 / 6 / 30 |

基线参考值只用于说明已存在的含义；小组运行底座不以代码默认值补齐缺失配置。模型名按获准网关实际模型填写，不把示例当可用账号或服务承诺。

## 密钥轮换流程

发现泄露时由仓库所有者与服务配置负责人先吊销并轮换相应密钥，再替换各环境本地配置，检查旧凭据已经失效，记录脱敏结果。随后评审历史清理方案并协同所有成员更新 clone；删除当前文件不能代替轮换。不能在公开 issue、截图、聊天记录、提交信息或报告里写原值。

检查范围为全部 Git 历史、本机待提交文件、构建上下文/镜像层、前端产物、日志与响应。自动检查只给出路径和规则，不回显匹配原值；检测到疑似值停止提交。机密原值不作为命令行参数传入，避免进入命令历史。
