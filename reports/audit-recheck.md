# 审计回查验证

日期：2026-11-16。HEAD `c0d35f0004f60a3a6ec7cac6827dc15061e9ba1a` 加本地工作树；源码及测试指纹 `92ad079a3cad936a48a28b19ee96acec52ffa1cfaee5d4a2defa9cdc7a2a4197`。未创建提交或推送。


`python3 apitest/run_all.py --case audit-events`及全量make check通过；本轮全量复放的学生403审计ID为8598。actor_id来自服务端会话，查询参数不能覆盖class_id。

| 前置条件与操作 | 实际结果 |
| --- | --- |
| A1无权读取审计；教师用actor_id=2及最近10分钟UTC窗查询 | 学生403，教师200，仅本班A1记录，403可回查 |
| 本班作业完成初判；按skill_id=grading-verdict-v3和时间窗查询 | 找到实际任务/作答对象、状态、技能、用量和耗时，尝试唯一键仅1条 |
| A教师读取B班审计及不存在ID | 404结构和文案相同；列表无B班记录 |
| page=0/1.5、未知字段、非法技能、重复参数、超过31天 | 400；page_size=101按100处理 |
| 分页期间增加审计 | snapshot_id/before_id固定上界，两页不重叠 |
| 普通教学、无技能、工具多轮、未报用量、503与取消 | 持久记录与stdout实际usage一致；缺失显式reported=false，失败/取消不记完成 |
| 审计预留/身份绑定失败、结束写失败、消费者故障恢复 | 前两者503且业务不变；结束失败保留started；无模型调用不消耗预算 |

七列：actor_id、time、action、object、result、skill_id、model.usage与duration_ms。附加request_id用于关联，不存凭据、正文、原始查询或模型输出。无模型时model/skill_id为null，不能把空用量解释成已上报零消耗。

接口例：`GET /api/audit-events?actor_id=2&from=2026-11-16T00:00:00Z&to=2026-11-16T23:59:59Z`；技能查询将actor_id替为skill_id。需要本班教师Bearer，详情为`GET /api/audit-events/{id}`。实际时间窗口必须涵盖本次运行。请求日志本身也留痕；当前查询不出现在自己的快照中。

教师audit状态键在375/768/1280px核对了布局、详情/返回、非法筛选错误及无命中状态；控件最小44px，窄屏记录换行为卡片。性能：5次预热后100次，p95=18.99ms。迁移两次及旧镜像20条样本回读一致。原始脱敏测试输出和迁移校验位于本机output/security-audit-20261116/。
