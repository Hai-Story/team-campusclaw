#!/usr/bin/env python3
import argparse,importlib.util,json,subprocess,sys,time,traceback
from pathlib import Path
from common import Context,ROOT

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--case',default='');args=parser.parse_args()
    context=Context();results=[]
    try:
        for path in sorted((Path(__file__).parent/'cases').rglob('*.py')):
            name=str(path.relative_to(Path(__file__).parent))
            if args.case and args.case not in name:continue
            start=time.monotonic()
            try:
                spec=importlib.util.spec_from_file_location('case_'+str(len(results)),path);module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
                observed=module.run(context);status='PASS'
            except Exception as error:
                status='FAIL';frame=traceback.extract_tb(error.__traceback__)[-1];observed={'error':type(error).__name__+': '+str(error),'line':frame.lineno,'file':Path(frame.filename).name}
            item={'case':name,'status':status,'seconds':round(time.monotonic()-start,3),'observed':observed};results.append(item)
            print(status,name,json.dumps(observed,ensure_ascii=False),flush=True)
    finally:context.cleanup()
    if not results:raise SystemExit('没有匹配的用例')
    out=ROOT/'output';out.mkdir(exist_ok=True)
    evidence={'commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT).decode().strip(),'worktree_changed':bool(subprocess.check_output(['git','status','--porcelain'],cwd=ROOT).strip()),'scope':'隔离MySQL、RabbitMQ与本地网关替身','results':results}
    (out/('checks-'+(args.case or 'all')+'.json')).write_text(json.dumps(evidence,ensure_ascii=False,indent=2)+'\n')
    return 1 if any(item['status']=='FAIL' for item in results) else 0
if __name__=='__main__':sys.exit(main())
