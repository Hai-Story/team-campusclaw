#!/usr/bin/env python3
"""Exercise SEC-01..04/07 against synthetic fixtures; do not execute SEC-08."""
import json,subprocess,sys,time,urllib.request,uuid
from common import Context,ROOT

def upload(c,marker):
    boundary='fixture'+uuid.uuid4().hex
    body=(f'--{boundary}\r\nContent-Disposition: form-data; name="title"\r\n\r\n安全隔离验证\r\n'
          f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="fixture.md"\r\nContent-Type: text/markdown\r\n\r\n'
          f'{marker} 是仅供班级A验证的虚构知识。\r\n--{boundary}--\r\n').encode()
    req=urllib.request.Request(c.base+'/api/materials',data=body,headers={'Authorization':'Bearer '+c.token('teacher'),'Content-Type':'multipart/form-data; boundary='+boundary})
    with urllib.request.urlopen(req,timeout=30) as response:
        assert response.status==201
        return json.load(response)['id']

def main():
    c=Context();observed={};material=None;conversation=None
    try:
        assignment=c.assignment('security-'+c.run_id)
        receipt=c.submit(assignment)[1];task=receipt['task_id'];submission=receipt['submission_id']
        c.wait(task)
        protected=[('GET','/api/me',None),('GET','/api/materials',None),('GET','/api/agents',None),
                   ('GET','/api/skills',None),('POST','/api/search',{'query':'方程'}),
                   ('POST',f'/api/assignments/{assignment}/submissions',{'answer':'2'}),
                   ('GET','/api/tasks/'+task,None),('GET','/api/submissions/'+str(submission),None),('PUT','/api/submissions/'+str(submission)+'/review',{'score':0,'correct':False,'expected_version':0})]
        statuses=[{'method':method,'path':path,'status':c.request(path,body,who=None,method=method)[0]} for method,path,body in protected]
        observed['SEC-01']={'status':'GREEN' if all(x['status']==401 for x in statuses) else 'RED','requests':statuses}

        marker='isolation'+uuid.uuid4().hex
        material=upload(c,marker)
        own=c.request('/api/search',{'query':marker,'mode':'keyword'})
        foreign=c.request('/api/search',{'query':marker,'mode':'keyword'},who='other')
        cross=c.request('/api/materials/'+str(material),who='other')
        missing=c.request('/api/materials/9223372036854775807',who='other')
        equal=cross[0]==missing[0]==404 and cross[1]==missing[1]
        own_hits=len(own[1].get('hits',[]));foreign_hits=len(foreign[1].get('hits',[]))
        observed['SEC-02']={'status':'GREEN' if equal and own[0]==foreign[0]==200 and own_hits>0 and foreign_hits==0 else 'RED',
                            'cross_status':cross[0],'missing_status':missing[0],'business_response_equal':equal,'own_keyword_hits':own_hits,'foreign_keyword_hits':foreign_hits,
                            'scope':'班级A教师上传随机唯一词，A1命中；班级B学生读取A资料及检索同一词'}

        skill=c.request('/api/skills',who='teacher')[1][0]
        skill=c.request('/api/skills/'+str(skill['id']),who='teacher')[1]
        agent=next(x for x in c.request('/api/agents',who='teacher')[1] if x['enabled'] and skill['id'] in x['skill_ids'])
        app=c.request('/api/ai-apps',who='teacher')[1][0]
        before=c.sql('SELECT id,SHA2(source_text,256) FROM tutor_skills ORDER BY id; SELECT id,SHA2(prompt,256) FROM tutor_agents ORDER BY id; SELECT class_id,app_key,version,SHA2(user_template,256) FROM tutor_app_templates ORDER BY class_id,app_key;')
        writes=[('PUT','/api/skills/'+str(skill['id']),{'source_text':skill['source_text']}),
                ('PUT','/api/agents/'+str(agent['id']),{key:agent[key] for key in ('name','prompt','enabled','skill_ids','material_ids')}),
                ('PUT','/api/ai-apps/'+app['key']+'/template',{'user_template':app['user_template'],'expected_version':app.get('template_version',0)})]
        codes=[{'method':method,'path':path,'status':c.request(path,body,method=method)[0]} for method,path,body in writes]
        after=c.sql('SELECT id,SHA2(source_text,256) FROM tutor_skills ORDER BY id; SELECT id,SHA2(prompt,256) FROM tutor_agents ORDER BY id; SELECT class_id,app_key,version,SHA2(user_template,256) FROM tutor_app_templates ORDER BY class_id,app_key;')
        observed['SEC-03']={'status':'GREEN' if all(x['status']==403 for x in codes) and before==after else 'RED','requests':codes,'database_unchanged':before==after}

        before=c.sql(f'SELECT COUNT(*) FROM grade_results WHERE submission_id={int(submission)}; SELECT COUNT(*) FROM submissions WHERE assignment_id={int(assignment)}; SELECT COUNT(*) FROM grade_reviews WHERE submission_id={int(submission)}; SELECT COUNT(*) FROM grade_review_history WHERE submission_id={int(submission)};')
        direct=c.request('/api/submissions/'+str(submission)+'/review',{'score':0,'correct':False,'expected_version':0},method='PUT')[0]
        injected=c.request(f'/api/assignments/{assignment}/submissions',{'answer':'2','score':60},headers={'Idempotency-Key':uuid.uuid4().hex})[0]
        cross_task=c.request('/api/tasks/'+task,who='other')[0]
        after=c.sql(f'SELECT COUNT(*) FROM grade_results WHERE submission_id={int(submission)}; SELECT COUNT(*) FROM submissions WHERE assignment_id={int(assignment)}; SELECT COUNT(*) FROM grade_reviews WHERE submission_id={int(submission)}; SELECT COUNT(*) FROM grade_review_history WHERE submission_id={int(submission)};')
        observed['SEC-04']={'status':'GREEN' if (direct,injected,cross_task)==(403,400,404) and before==after else 'RED',
                            'student_final_review':direct,'client_score_field':injected,'foreign_task_read':cross_task,'database_unchanged':before==after,
                            'scope':'学生调用真实教师终评接口被拒，AI只写初判；分数仅经教师终评写入'}

        c.control(delay_ms=10,statuses=[])
        code,data,_,_=c.request('/api/conversations',{'agent_id':agent['id']});assert code==201
        conversation=data['id']
        code,stream,_,headers=c.request(f'/api/conversations/{conversation}/messages',{'content':'方程怎么解，请给我下一步思路。','capture_trace':False})
        request_id=next((value for key,value in headers.items() if key.lower()=='x-request-id'),'')
        assert code==200 and isinstance(stream,str) and 'event: done' in stream,'教学技能调用未完成'
        time.sleep(.2)
        logs=c.compose('logs','--no-color','--no-log-prefix','api')
        matches=[]
        for line in logs.splitlines():
            try:item=json.loads(line)
            except ValueError:continue
            if item.get('request_id')==request_id:matches.append(item)
        fields=sorted(set().union(*(set(item) for item in matches))) if matches else []
        missing_fields=sorted({'actor_id','skill_id','status','duration_ms','usage'}-set(fields))
        selected='解题引导' in stream
        observed['SEC-07']={'status':'RED' if missing_fields or not selected else 'GREEN','skill_selected':selected,'request_log_count':len(matches),'request_log_fields':fields,'missing_fields':missing_fields,
                            'scope':'已完成的教学技能SSE请求；可选的私有调试trace不替代结构化审计日志'}
        grading=[]
        for line in c.compose('logs','--no-color','--no-log-prefix','worker').splitlines():
            try:item=json.loads(line)
            except ValueError:continue
            if item.get('task_id')==task and item.get('status')=='succeeded':grading.append(item)
        observed['SEC-07']['grading_log_fields']=sorted(set().union(*(set(item) for item in grading))) if grading else []
        observed['SEC-07']['grading_required_fields_present']=any({'actor_id','skill_id','status','duration_ms','usage'}<=set(item) for item in grading)
    finally:
        if conversation is not None:c.request('/api/conversations/'+str(conversation),method='DELETE')
        if material is not None:
            detail=c.request('/api/materials/'+str(material),who='teacher')[1]
            c.request('/api/materials/'+str(material),{'version':detail['version']},who='teacher',method='DELETE')
        c.cleanup()
    out=ROOT/'output';out.mkdir(exist_ok=True)
    evidence={'commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT).decode().strip(),'scope':'隔离MySQL/RabbitMQ/本地网关替身，合成数据','checks':observed}
    (out/'security-precheck.json').write_text(json.dumps(evidence,ensure_ascii=False,indent=2)+'\n')
    for key,value in observed.items():print(key,json.dumps(value,ensure_ascii=False))
    return 1 if any(value['status']=='RED' for value in observed.values()) else 0

if __name__=='__main__':sys.exit(main())
