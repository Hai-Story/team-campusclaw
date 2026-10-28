---
name: campusclaw-add-api
description: 按 CampusClaw 已评审的 OpenSpec 场景新增一个 Go API；用于代码阶段单个接口任务，不用于从候选清单直接生成整套功能。
---

# campusclaw-add-api

输入：change、单个task、接口契约与允许修改路径。

读AGENTS.md和产品/后端/安全/测试规则，复述proposal范围与Non-goals，严格校验change。若只有候选表或关键字段/状态/失败行为未定义，先修订规约，不能猜测实现。

先找真实路由、服务与数据访问层；保持既有/api、响应与错误契约。新接口显式登记权限，身份与班级取服务端。边界做语法校验，服务做业务规则，存储层承担唯一性与并发；外部调用走适配，不能夹在数据库事务中。

只实现当前task，补其Scenario对应测试并运行verify。审查diff后才勾选；提交写Change/Verify/Evidence。没有实际通过时不称完成，不顺带重构认证、迁移全部旧接口或实现其余候选模块。
