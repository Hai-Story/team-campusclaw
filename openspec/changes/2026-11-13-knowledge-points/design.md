## Context

现有/api/knowledge相关对象是资料切片，没有业务知识点。动机见proposal.md；本变更只实现K1-K3。

## Goals / Non-Goals

**Goals:** 新增独立curriculum模块，边界/服务/存储分层，复用认证与学科枚举。

**Non-Goals:** 不改旧作业表、不修改会话权限，不预建其余工作包空壳。

## Decisions

- 新增knowledge_points、knowledge_point_materials、knowledge_point_assignments、knowledge_favorites及curriculum_migrations。关联以外键清理，本班/同学科校验在写事务内执行；名称唯一与收藏唯一由数据库保证。
- 使用事务内锁行、expected_version防止丢失更新；不以最后写入覆盖。非法关联整笔回滚，不逐条部分保存。
- 迁移使用独立命名锁与只增版本，重跑安全。回退旧版保留新增表；不通过删表回滚。
- 页面沿用App状态导航和视觉令牌。关联资源显示标题并链接原页面；作业跳转保存目标ID，并通过原单题读取接口补齐分页外目标，失败不选用其他作业；不会新建虚构URL路由。
- 证据集中在reports/learning-expansion.md；各轮只追加真实结果。无新增依赖。

## Risks / Trade-offs

- [知识点被并发修改] → 锁行与版本冲突，页面提示重新加载。
- [切学科后旧请求返回] → AbortController与请求代次隔离。

## Migration Plan

API启动在原grading迁移之后执行独立增量迁移；隔离库检查重跑及旧表计数不变。回退只恢复旧镜像，保留新增表与记录。
