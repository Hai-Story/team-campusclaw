# 安全门禁红绿证明

日期：2026-11-16。HEAD `c0d35f0004f60a3a6ec7cac6827dc15061e9ba1a` 加本地工作树；源码及测试指纹 `92ad079a3cad936a48a28b19ee96acec52ffa1cfaee5d4a2defa9cdc7a2a4197`。未创建提交或推送。


| 运行 | 固定输入/范围 | 实际退出与输出 |
| --- | --- | --- |
| 红 | `SECURITY_PROVE_RED=1 make check-security`；临时目录内随机生成的合成扫描凭据，从未作为真实密钥使用，不写入工作树 | gitleaks退出1，make退出2；`{"gate":"RED","synthetic_fixture":true,"scanner_exit":1}` |
| 绿 | `make check`内调用正常`make check-security`；完整可达历史、工作树、运行前端、所选Compose日志，npm及backend/mcp-server扫描 | make退出0；SEC-05命中0，npm全部等级0，Go可达通告0/0，gate=GREEN |

红反例走真实gitleaks，不把固定return1当作扫描证明；正常门禁没有跳过检查。合成反例不进入常规工作树或资产报告。受测Compose项目见[资产基线](security-baseline.md)。两次输出留于本机output/security-audit-20261116/gate-red.log与full-check.log；脚本返回码另有gate-red.json。代码保持未提交，因此此处记录实际HEAD和工作树指纹，不虚构“修复提交号”。
