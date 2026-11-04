# 配置字典与机密边界

`.env.example`只保留键名。`scripts/dev_env.py`生成专用验证值和随机凭据；服务启动校验必填项，缺失不以代码默认值补齐。部署值保存在本机`.env`，不进入镜像或版本库。

| 键 | 含义与校验 |
| --- | --- |
| COMPOSE_PROJECT_NAME | 隔离Compose命名；自动接口测试只接受campusclaw-team-async |
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
| MAX_UPLOAD_BYTES | 单文件上限，必填正整数；参考值 1048576 |
| SESSION_TTL_HOURS | 会话有效小时数，必填正数；参考值 24 |
| LOGIN_FAIL_LIMIT / LOGIN_FAIL_WINDOW_SECONDS | 登录限流阈值/窗口，必填正整数；参考 5 / 300 |
| CHUNK_AUTO_MAX_CHARS / CHUNK_AUTO_OVERLAP | 切片长度/重叠；前者正整数，后者非负且小于长度；参考 800 / 80 |
| VECTOR_SCORE_THRESHOLD / RRF_K / ASK_TOP_K | 检索阈值 0 到 1，融合 k 与命中数正整数；参考 0.25 / 60 / 4 |
| MCP_MAX_ROUNDS / MCP_MAX_CALLS_PER_TURN / MCP_MAX_CALLS_PER_CONVERSATION | 工具轮数/单轮与会话调用预算，必填正整数；参考 3 / 6 / 30 |

| RABBITMQ_USER / RABBITMQ_PASSWORD | 代理账号；必填，随机口令，仅内部网络 |
| AMQP_URL | 含认证的amqp/amqps地址，必填，不记录连接串；与代理凭据一致 |
| GRADE_TIMEOUT_SECONDS | 单次模型预算，整数1至600秒；验证5，部署建议60 |
| GRADE_MAX_ATTEMPTS | 总执行上限，整数1至10；验证与部署建议3 |
| GRADE_LEASE_SECONDS | 任务租约，整数2至3600秒且大于模型预算；验证8，部署建议70 |
| GRADE_RETRY_BASE_MS | 指数退避基数，整数10至30000毫秒；验证1000，附加0至20%抖动 |
| OUTBOX_POLL_MS | 发件箱轮询间隔，整数50至60000毫秒；验证200 |

验证网关为`http://gateway:8089/v1`，嵌入维度8；CHAT/EMBEDDING模型分别为fixture-chat/fixture-embedding。这些值仅用于本地替身，真实部署须填写获准服务的模型名与维度，并按实际预算设置超时及租约。

`.secrets/mysql-root`是数据库管理口令文件，目录权限700、文件600，仅挂载数据库；API只使用普通数据库账号。它与`.env`均不入库，不能在已有数据卷上仅重建文件来代替数据库内的口令轮换。

## 密钥轮换流程

发现泄露时由仓库所有者与服务配置负责人先吊销并轮换相应密钥，再替换各环境本地配置，检查旧凭据已经失效，记录脱敏结果。随后评审历史清理方案并协同所有成员更新 clone；删除当前文件不能代替轮换。不能在公开 issue、截图、聊天记录、提交信息或报告里写原值。

检查范围为全部 Git 历史、本机待提交文件、构建上下文/镜像层、前端产物、日志与响应。自动检查只给出路径和规则，不回显匹配原值；检测到疑似值停止提交。机密原值不作为命令行参数传入，避免进入命令历史。
