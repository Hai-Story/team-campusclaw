#!/usr/bin/env python3
"""Capture real SEC-05/06 baselines; findings keep the gate nonzero."""
from pathlib import Path
import datetime,json,shutil,subprocess,sys,tempfile

ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'output/security'
GO='golang:1.23-alpine'

def run(args,**kwargs):
    return subprocess.run(args,cwd=ROOT,capture_output=True,text=True,**kwargs)

def objects(text):
    decoder=json.JSONDecoder();offset=0
    while offset<len(text):
        while offset<len(text) and text[offset].isspace():offset+=1
        if offset==len(text):break
        value,offset=decoder.raw_decode(text,offset);yield value

def ensure_tools():
    check=run(['docker','run','--rm','-v','campusclaw-team-tools:/tools',GO,'sh','-c','test -x /tools/gitleaks && test -x /tools/govulncheck'])
    if check.returncode:
        install=run(['docker','run','--rm','-v','campusclaw-team-tools:/tools','-v','campusclaw-team-gomod:/go/pkg/mod','-e','GOBIN=/tools',GO,'sh','-c','go install github.com/zricethezav/gitleaks/v8@v8.24.3 && go install golang.org/x/vuln/cmd/govulncheck@v1.1.4'],timeout=600)
        if install.returncode:raise RuntimeError('安全工具安装失败；未产生扫描结论')

def scan_secrets():
    with tempfile.TemporaryDirectory(prefix='campusclaw-security-') as folder:
        snapshot=Path(folder)
        for name in ('history','worktree','frontend','logs'):(snapshot/name).mkdir()
        # Scan every distinct reachable blob, including deleted files, without needing Git in the scanner image.
        seen=set();blob_count=0
        for line in subprocess.check_output(['git','rev-list','--objects','--all'],cwd=ROOT).decode().splitlines():
            if ' ' not in line:continue
            oid,name=line.split(' ',1)
            if oid in seen:continue
            seen.add(oid)
            if subprocess.check_output(['git','cat-file','-t',oid],cwd=ROOT).strip()!=b'blob':continue
            rel=Path(name)
            if rel.is_absolute() or '..' in rel.parts:raise RuntimeError('历史路径不合法')
            target=snapshot/'history'/oid/rel;target.parent.mkdir(parents=True,exist_ok=True)
            target.write_bytes(subprocess.check_output(['git','cat-file','blob',oid],cwd=ROOT));blob_count+=1
        messages=subprocess.check_output(['git','log','--all','--format=%B'],cwd=ROOT)
        (snapshot/'history/commit-messages.txt').write_bytes(messages)
        names=subprocess.check_output(['git','ls-files','--cached','--others','--exclude-standard','-z'],cwd=ROOT).decode().split('\0')
        for name in filter(None,names):
            source=ROOT/name
            if not source.is_file():continue
            if source.is_symlink():raise RuntimeError('扫描前须明确符号链接的范围')
            target=snapshot/'worktree'/name;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(source,target)
        container=run(['docker','compose','ps','-q','web']).stdout.strip()
        if not container:raise RuntimeError('web未运行，前端产物尚未扫描')
        copied=run(['docker','cp',container+':/usr/share/nginx/html/.',str(snapshot/'frontend')])
        if copied.returncode:raise RuntimeError('无法读取正在运行的前端产物')
        logs=run(['docker','compose','logs','--no-color','--no-log-prefix'])
        if logs.returncode:raise RuntimeError('运行日志读取失败')
        (snapshot/'logs/runtime.log').write_text(logs.stdout+logs.stderr)
        root_markers=(logs.stdout+logs.stderr).count('GENERATED ROOT PASSWORD:')
        scans={}
        for scope in ('history','worktree','frontend','logs'):
            report=OUT/f'gitleaks-{scope}.json'
            command=['docker','run','--rm','-v','campusclaw-team-tools:/tools:ro','-v',str(snapshot)+':/scan:ro','-v',str(OUT)+':/out',GO,'/tools/gitleaks','dir','/scan/'+scope,'--no-banner','--redact','--log-level','error','--report-format','json','--report-path','/out/'+report.name]
            result=run(command,timeout=180)
            if result.returncode not in (0,1) or not report.exists():raise RuntimeError('密钥扫描失败: '+scope)
            findings=json.loads(report.read_text());scans[scope]={'hits':len(findings),'exit_code':result.returncode}
        scans['logs']['initialization_credential_markers']=root_markers
        return {'historical_blobs':blob_count,'scans':scans,'hits':sum(x['hits'] for x in scans.values())+root_markers}

def audit():
    frontend=run(['npm','--prefix','frontend','audit','--json'],timeout=180)
    (OUT/'npm-audit.json').write_text(frontend.stdout)
    npm=json.loads(frontend.stdout)
    if 'error' in npm:raise RuntimeError('npm审计未完成')
    go={}
    for module in ('backend','mcp-server'):
        command=['docker','run','--rm','-v',str(ROOT/module)+':/src:ro','-v','campusclaw-team-gomod:/go/pkg/mod','-v','campusclaw-team-gocache:/root/.cache/go-build','-v','campusclaw-team-tools:/tools:ro','-w','/src',GO,'/tools/govulncheck','-json','./...']
        result=run(command,timeout=240);(OUT/f'govulncheck-{module}.json').write_text(result.stdout)
        if result.returncode not in (0,3):raise RuntimeError('Go依赖审计未完成: '+module)
        stream=list(objects(result.stdout))
        findings=[entry['finding'] for entry in stream if 'finding' in entry]
        reachable=sorted({entry['osv'] for entry in findings if entry.get('trace') and entry['trace'][0].get('function')})
        go[module]={'exit_code':result.returncode,'reachable_advisories':reachable,'reachable_count':len(reachable),'severity':'工具未提供CVSS高危/严重分级，不能计为0'}
    packages={name:{'severity':value['severity'],'direct':value.get('isDirect',False),'range':value.get('range','')} for name,value in npm.get('vulnerabilities',{}).items()}
    return {'npm':{'exit_code':frontend.returncode,'counts':npm['metadata']['vulnerabilities'],'packages':packages},'go':go}

def write_report(data):
    reports=ROOT/'reports';reports.mkdir(exist_ok=True)
    lines=['# 安全资产基线','',f"检查日期：{data['date']}。提交：`{data['commit']}`。",'',
           '扫描范围：全部可达Git历史的唯一文件内容与提交说明、当前工作树、运行中web容器的前端产物、隔离Compose全部服务日志。只报告脱敏结果。','',
           '## SEC-05 密钥扫描','', '`make check-security` 调用固定版本gitleaks v8.24.3；历史包含已删除文件。','',
           '| 范围 | 命中数 | 退出码 |','| --- | --- | --- |']
    for scope,value in data['secrets']['scans'].items():lines.append(f"| {scope} | {value['hits']} | {value['exit_code']} |")
    lines+=['',f"历史扫描覆盖{data['secrets']['historical_blobs']}个唯一文件内容；初始化口令日志标记{data['secrets']['scans']['logs']['initialization_credential_markers']}处。"]
    rotation=OUT/'root-rotation.json'
    if rotation.exists():lines+=['','隔离数据库初始化日志曾命中1处管理口令；已轮换并验证旧值失效，重新创建容器后当前日志复扫。初始化改用仅数据库可读的秘密文件，明文未进入Git历史。']
    lines+=['','## SEC-06 依赖审计','', '`npm --prefix frontend audit --json` 的计数按受影响包统计：','', '```json',json.dumps(data['dependencies']['npm'],ensure_ascii=False,indent=2),'```','', '`govulncheck -json ./...` 分别扫描backend与mcp-server，固定扫描器v1.1.4；可达通告按唯一GO编号去重，JSON模式退出0不等同无漏洞。','']
    for name,item in data['dependencies']['go'].items():
        lines+=[f"- {name}：退出{item['exit_code']}，可达通告{item['reachable_count']}项；{item['severity']}。"]
        if item['reachable_advisories']:lines+=['  '+', '.join(item['reachable_advisories'])]
    lines+=['','命令末行原文：','', '```json', json.dumps({'SEC-05_hits':data['secrets']['hits'],'npm_counts':data['dependencies']['npm']['counts'],'go_reachable':{name:value['reachable_count'] for name,value in data['dependencies']['go'].items()},'gate':'RED' if data['red'] else 'GREEN'},ensure_ascii=False), '```','','## 判定与处置','',f"本次资产门禁：**{'红' if data['red'] else '绿'}**，入口退出码{1 if data['red'] else 0}。",'',
            '已发现的依赖问题登记为待处置：后续变更升级Go构建链及受影响前端依赖，复跑构建、功能与依赖检查后再评审；登记不表示问题已修复或获准放行。','',
            '完整脱敏扫描输出由命令生成在本机output/security/；本文件保留计数、退出码与通告编号，可运行相同入口复核。']
    (reports/'security-baseline.md').write_text('\n'.join(lines)+'\n')

def main():
    OUT.mkdir(parents=True,exist_ok=True);ensure_tools();secrets=scan_secrets();dependencies=audit()
    red=bool(secrets['hits'] or dependencies['npm']['counts'].get('high',0) or dependencies['npm']['counts'].get('critical',0) or any(value['reachable_count'] for value in dependencies['go'].values()))
    data={'date':datetime.datetime.now(datetime.timezone.utc).date().isoformat(),'commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT).decode().strip(),'secrets':secrets,'dependencies':dependencies,'red':red}
    (OUT/'baseline-summary.json').write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n');write_report(data)
    print(json.dumps({'SEC-05_hits':secrets['hits'],'npm_counts':dependencies['npm']['counts'],'go_reachable':{name:value['reachable_count'] for name,value in dependencies['go'].items()},'gate':'RED' if red else 'GREEN'},ensure_ascii=False))
    return 1 if red else 0
if __name__=='__main__':sys.exit(main())
