# 仓库所有者与成员待办

仓库：https://github.com/Hai-Story/team-campusclaw 。当前仓库写入连接具有push权限，管理权限为false；读取保护详情返回权限限制。随后通过可读的分支与规则集接口核实：main的protected=false、protection.enabled=false，rulesets为空；本次检查时尚未启用主干保护。

## 成员加入

由Hai-Story在Settings → Collaborators邀请成员对应的GitHub账号，三人本人接受后核对名单。姓名与账号的对应由本人确认，不用显示名猜测。公开仓库供教师读取；若调整可见性须重新核对教师访问权限。

## main保护（业务代码前完成）

所有者在Settings → Rules → Rulesets（或Branches的保护规则）对main启用并保存：

- Require a pull request before merging；至少1名非作者批准。
- 新提交使旧批准失效；评审会话需解决后合并。
- 禁止强制推送和删除main；管理员遵守或使用明确且可审计的紧急流程。
- 后续真实CI建立后，再把实际产生的check名称设为必需，不能填写不存在的检查并声称门禁已生效。

第7课没有应用代码，先落实PR/评审保护即可；CI必须在运行门禁任务实施后补接。保存后核对规则为Active、目标确实匹配main，仓库套餐允许实际生效。

## 生效验证与记录

成员用临时分支提交一处文档变更并开PR，检查作者不能代替所需的非作者批准；若平台提供规则评估/规则洞察，保存其结论。必要时在可安全回退的文档提交上验证直接更新main被拒绝，避免用正式业务改动作探针。核对force-push和delete已被保护。

记录日期、执行人、目标分支、规则名称、评审/合并表现与平台结果；截图由用户自行完成。助手不能把操作指南写成设置完成。现有LICENSE初始提交保留，第7课.gitignore先于任何应用代码入库，不重写他人历史。
