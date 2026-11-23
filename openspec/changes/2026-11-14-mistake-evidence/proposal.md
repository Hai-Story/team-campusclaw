## Why

错题讲解需要真实教学依据与可回查出处，生成期间的改判和资料变化不能留下过期结果。

## What Changes

- 保留原讲解入口、explanation字段与取消重试，增加真实出处及依据不足状态。
- 本班检索后验证来源与答案边界，返回前重查错题和资料。

## Capabilities

### New Capabilities
- `mistake-evidence`: 错题讲解的检索与安全出处。

### Modified Capabilities

无。

## Impact

grading讲解、检索注入、学生讲解组件与接口文档。无新表、无新增依赖；不改变初判或终评。
