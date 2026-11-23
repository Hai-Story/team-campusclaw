# 安全资产基线

检查日期：2026-11-23。提交：`a708dbf18fba28ee935c704f818068970b13a596`。

扫描范围：全部可达Git历史的唯一文件内容与提交说明、当前工作树、运行中web容器的前端产物、所选Compose项目全部服务日志。只报告脱敏结果。

受测Compose项目：`campusclaw-team-test`。

## SEC-05 密钥扫描

`make check-security` 调用固定版本gitleaks v8.24.3；历史包含已删除文件。

| 范围 | 命中数 | 退出码 |
| --- | --- | --- |
| history | 0 | 0 |
| worktree | 0 | 0 |
| frontend | 0 | 0 |
| logs | 0 | 0 |

历史扫描覆盖711个唯一文件内容；初始化口令日志标记0处。

## SEC-06 依赖审计

`npm --prefix frontend audit --json` 的计数按受影响包统计：

```json
{
  "exit_code": 0,
  "counts": {
    "info": 0,
    "low": 0,
    "moderate": 0,
    "high": 0,
    "critical": 0,
    "total": 0
  },
  "packages": {}
}
```

`govulncheck -json ./...` 分别扫描backend与mcp-server，固定扫描器v1.9.0；可达通告按唯一GO编号去重，JSON模式退出0不等同无漏洞。

- backend：退出0，可达通告0项；工具未提供CVSS高危/严重分级，不能计为0。
- mcp-server：退出0，可达通告0项；工具未提供CVSS高危/严重分级，不能计为0。

命令末行原文：

```json
{"SEC-05_hits": 0, "npm_counts": {"info": 0, "low": 0, "moderate": 0, "high": 0, "critical": 0, "total": 0}, "go_reachable": {"backend": 0, "mcp-server": 0}, "gate": "GREEN"}
```

## 判定与处置

本次资产门禁：**绿**，Python脚本退出码0（make目标退出0）。

本轮密钥及依赖检查通过；功能与成员验收分别记录。

完整脱敏扫描输出由命令生成在本机output/security/；本文件保留计数、退出码与通告编号，可运行相同入口复核。
