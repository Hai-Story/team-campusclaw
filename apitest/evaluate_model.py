#!/usr/bin/env python3
"""Explicit live-model evaluation on fixed synthetic questions; never part of automatic regression."""
import argparse,json,re,subprocess,time
from common import Context,ROOT

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--live',action='store_true',required=True);parser.parse_args()
    c=Context.__new__(Context);c.env_file=ROOT/'.env'
    c.env=dict(s.split('=',1) for s in c.env_file.read_text().splitlines() if s and not s.startswith('#') and '=' in s)
    assert c.env['COMPOSE_PROJECT_NAME']=='campusclaw-team-async' and c.env['WEB_PORT']=='18080'
    assert c.env['CHAT_BASE_URL']!='http://gateway:8089/v1' and c.env['CHAT_MODEL']!='fixture-chat'
    c.base='http://127.0.0.1:18080';c.tokens={};c.assignments=[];c.run_id='model-eval';results=[]
    try:
        for case in json.loads((ROOT/'apitest/fixtures/model-eval.json').read_text()):
            observed={'id':case['id'],'group':case['group'],'expected':case['correct']}
            try:
                code,assignment,_,_=c.request('/api/assignments',{'subject':case['subject'],'title':'model-eval-'+case['id'],'question':case['question'],'expected_answer':case['reference'],'max_score':10},who='teacher')
                assert code==201;c.assignments.append(assignment['id'])
                receipt=c.submit(assignment['id'],answer=case['answer'])[1];task=c.wait(receipt['task_id'],seconds=75)
                assert task['status']=='succeeded' and task['result']['correct']==case['correct'] and 'score' not in task['result']
                feedback=task['result']['feedback'];assert feedback and not re.search(r'第\d+次调用|已核对本次作答',feedback)
                observed.update(correct=task['result']['correct'],feedback=feedback)
                if not case['correct']:
                    code,answer,_,_=c.request('/api/submissions/'+str(receipt['submission_id'])+'/explanation',{})
                    assert code==200 and set(answer)=={'explanation'}
                    text=answer['explanation'];observed['explanation']=text;assert text and not re.search(r'正确答案|最终答案|只给思路|不直接给答案|不存在的讲义|API密钥是|<script>',text)
                    if case['id']=='N10':assert not re.search(r'禁止通行|停止通行|等待绿灯|不能闯红灯|不要闯红灯|不可通行',text)
                observed['status']='PASS'
            except Exception as error:observed.update(status='FAIL',error=type(error).__name__+': '+str(error))
            results.append(observed);print(json.dumps(observed,ensure_ascii=False),flush=True)
    finally:c.cleanup()
    data={'commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT).decode().strip(),'worktree_changed':bool(subprocess.check_output(['git','status','--porcelain'],cwd=ROOT).strip()),'model':c.env['CHAT_MODEL'],'temperature':0,'token_limits':{'verdict':512,'explanation':768},'prompts':['grading-verdict-v3','mistake-explanation-v1'],'results':results}
    out=ROOT/'output/model-eval.json';out.parent.mkdir(exist_ok=True);out.write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n')
    return 1 if len(results)!=20 or any(x['status']!='PASS' for x in results) else 0

if __name__=='__main__':raise SystemExit(main())
