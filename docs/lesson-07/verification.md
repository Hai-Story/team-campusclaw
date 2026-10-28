# 本课验证记录

日期：2026-10-28（Asia/Shanghai）。范围：启动文档、指令文件、规约与目录说明；业务源码和运行行为属于后续任务。

## 已执行

| 检查 | 方法/命令 | 结果与范围 |
| --- | --- | --- |
| 个人聊天与来源 | read_thread读取“互联网软件开发（个人任务）”，核对本地来源提交、README、配置和入口 | 固定e5ffbb1677dfad4f5347a4023dc88a5b607c429a，GitHub提交API也确认该对象存在；只作源码事实核对 |
| 选题说明 | 提取3页正文并逐页查看PDF渲染 | 七项扩展、技术方案、三人分工已核对 |
| 课程页面 | Chrome原生页面及HTTP正文/最终网址核对 | references列出的主页面与15个子页面均有正文，非只看标题 |
| OpenSpec初始化 | openspec init --tools codex --language zh-CN | 退出0，生成6个Codex工作流技能；附带跳过Copilot选项不影响结果 |
| 根目录 | openspec context --json | 退出0，解析为小组仓库，无全局store误指向 |
| 规约形式 | openspec validate 2026-10-28-team-project-kickoff --strict | 退出0，Change is valid |
| 四件套状态 | openspec status --change 2026-10-28-team-project-kickoff | 退出0，4/4 artifacts complete；只代表规划完整 |
| 仓库权限 | GitHub get_repo / authenticated user | MrKen20可push，admin/maintain=false |
| 主干与规则集 | GET branches/main、GET rulesets | protected=false，protection.enabled=false，rulesets=[]；所有者待设置 |
| 管理详情访问 | GET branches/main/protection | 403 Resource not accessible by integration；没有宣称已修改设置 |

技能、路径与机密检查结果在完成后补入下节；所有结论仅对应记录中的实际动作。

## 文档门禁结果

| 检查 | 实际执行与结果 |
| --- | --- |
| 技能格式 | skill-creator的quick_validate.py逐一检查13个目录，最终13/13退出0；结果见check-results.json |
| 验证器环境 | 首次缺少PyYAML；在仓库外临时验证目录安装PyYAML 6.0.3后重跑，未增加应用依赖 |
| OpenSpec技能元信息 | 自动生成6份技能的顶层compatibility不在当前检查器支持字段内；保留说明并移入metadata后重跑通过，正文未改 |
| 文档内部链接 | 逐条解析并验证20个相对Markdown链接，0断链；8份规则加载路径可读 |
| 配置与忽略 | 35行均为唯一的KEY=，无实值；7个本机/产物路径被忽略，.env.example可跟踪 |
| 规约映射 | 5条Requirement、12个唯一Scenario，每条均有GIVEN/WHEN/THEN且在映射表可找到 |
| 文档边界 | 核对52个现有文件仅为Markdown、OpenSpec配置、忽略/占位和LICENSE；没有应用源码、脚本或容器编排 |
| 机密模式检查 | 当前文件及本地全部可达历史（初始LICENSE的1个blob）按私钥、常见令牌、非空机密赋值和.env路径规则检查，0发现；扫描器正反例自检通过 |
| diff | git diff --cached --check退出0，人工核对提交文件清单与分层、候选/事实、Non-goals一致性 |

check-results.json保留上述检查的数据结果。机密检查是所列模式和路径的静态检查，不代表运行镜像/外部日志审计。初次空赋值扫描跨行误报已修正并经正反例重跑，原始配置始终只有空值。

## 尚待执行的验收

代码行为S1-S12、统一make check、真实模型/工具评测、新会话生成代码、违规注入、组员人审、异机启动、回滚及运行标签均未在本次文档任务中执行。任务勾选保留在对应阶段，没有把个人历史报告搬成小组新验证。

## 发布核对

待记录。GitHub回读成功后记入实际提交与文件树对比结果。
