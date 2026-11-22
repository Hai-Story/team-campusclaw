# 设计

1. curriculum的010迁移只新增student_homework_notifications：id/class_id/created_by/student_id、assignment_id或paper_id二选一、created_at/updated_at/read_at。每学生每资源唯一；用户和资源删除级联清理；首次启用不为旧作业补发。旧应用可忽略新表，回退保留新表，重新升级可继续读取。
2. 单题发布、整卷发布、显式示例单题导入在同一数据库事务内，INSERT SELECT为当前班全部学生创建提醒。整卷只提醒一次，逐题不提醒；发布失败回滚作业及提醒；整卷幂等复用不再次生成提醒。
3. GET /api/homework-notifications仅学生，默认page=1/page_size=20，上限100、按id倒序，返回items/page/page_size/unread_count。每条只含id/kind/resource_id/subject/title/created_at/read_at，不含答案、分数或其他学生数据。count和列表使用同一只读事务快照。
4. PUT /api/homework-notifications/{id}/read无请求体，幂等204。只匹配本人及当前班，越权与不存在同形404；非法id或分页400，未登录401，教师403，依赖故障503；不得相信客户端student_id/class_id。
5. “作业提醒”按钮和面板仅在学生“提交作业”的单题/多题视图显示，其他页面只在左侧“提交作业”旁显示未读数量圆标；无未读时隐藏圆标，教师不显示。切离作业视图关闭面板，轮询继续更新左侧数量；同一数据源同步面板与圆标，标为已读后同时减少数量。初次/聚焦/前台每30秒读取，页面隐藏时暂停、禁止请求重叠，卸载取消；失败保留数量并显示错误，恢复后重试。登出/换账号清除本地提醒状态。
6. 提醒面板提供最新20条及分页、刷新、加载/空/错误/重试；标为已读和打开作业独立操作，打开时先保存已读再跳转，不自动改写作答。非模态section可用Escape关闭并恢复焦点，移动端宽度不溢出，按钮44px；左侧圆标复用间隔复习样式。两类作业使用App现有openAssignment/openPaper。

验证：隔离A/B班与两名同班学生，验证发布原子性/并发已读/多题幂等/删除/新旧应用回退/重启；100次列表p95≤500ms（20条提醒、预热5次、并发1、HTTP端到端）。真实AI复测沿用九科change及其固定20+20用例，独立执行不影响18080数据。
