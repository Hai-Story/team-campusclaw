# 并发与重投复现

> 来源记录：以下 2026-11-05 内容随 `codex/homework-teacher-review@df755cc` 交接导入，保留原受测提交与执行范围，不代表张海峰本次独立复测。2026-11-07 的集成与独立复测结果见 [review-01.md 的追加记录](review-01.md#2026-11-07-张海峰集成与独立复测)。

日期：2026-11-05。受测HEAD：`ba61ce5f639fa45f40e566be171e26b67bc42847`，含本次回归更改；接口运行器退出0。

手段：`apitest/cases/api/tasks/GET.py`操作真实RabbitMQ与MySQL，停止/重启、强制终止worker、重复发布task_id，并用两个消费者与过期租约制造旧执行者回写竞争。数据全部为本次创建的合成作业。

对象：任务持久性、重复执行、结果唯一、租约接管与模型等待期间的API可用性（A3-S1/S2/S3）。

判据：停机时pending且无结果，恢复60秒内成功；中断后attempts增加，重复投递仍仅一份结果；旧run_token不得覆盖新结果；查询与新提交<1秒。

```bash
python3 apitest/run_all.py --case tasks
```

实际输出摘要：`stopped_consumer="pending then succeeded"`、`fenced_attempts=2`、`stale_writer="discarded"`、`crash_attempts=2`、`duplicate_results=1`，阻塞期间查询/提交均<1秒。完成后再发布同一task_id，AI判断与反馈不变。判定：**通过**。

失败观测：503/503/200在attempts=3成功；持续503在3次后failed，401在1次后failed，无成绩结果；两条本次死信事件均确认published。grade.dead保留累积消息，数量可因复测增加，不能把全队列累计值当作本次失败数量。查看`docker compose exec -T worker /app/grade-worker -queues`获取当前就绪数和消费者数；该命令不消费死信。

用例结束恢复一个worker及正常网关，不清空共享队列，不删除非本次数据。只保证本场景的一份生效结果，不宣称消息恰好投递一次或生产高可用。

终评并发另验：`python3 apitest/run_all.py --case review`，同一expected_version两请求实测200/409，仅成功版本追加历史；先终评后AI返回、完成后再重投均不覆盖教师结果。
