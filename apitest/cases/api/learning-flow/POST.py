"""I1: one disposable class-scoped learning workflow, including final-only statistics."""
import json,time,uuid,urllib.request

def run(c):
    prefix='flow-'+c.run_id;users=[];point_ids=[];material=lesson=draft=paper=published=walk=graphdraft=server=None;children=[]
    tables='materials,grade_assignments,submissions,grade_reviews,knowledge_points,lesson_plans,question_drafts,bank_questions,paper_drafts,published_papers,paper_attempts'
    before=c.sql('CHECKSUM TABLE '+tables)
    def req(path,body=None,who='flow_teacher',method=None,timeout=130):return c.request(path,body,who,method,timeout=timeout)[:2]
    def student(path,body=None,method=None):return req(path,body,'flow_student',method)
    def wait(task):
        for _ in range(400):
            v=student('/api/tasks/'+task)[1]
            if v['status'] in ['succeeded','failed']:assert v['status']=='succeeded',v;return v
            time.sleep(.1)
        raise AssertionError('initial judgment did not finish')
    try:
        for role,seed in [('teacher','teacher_a'),('student','student_a1')]:
            username=prefix+'-'+role;c.sql("INSERT INTO users(username,password_hash,role,class_id) SELECT '"+username+"',password_hash,role,class_id FROM users WHERE username='"+seed+"'");uid=int(c.sql("SELECT id FROM users WHERE username='"+username+"'")[0][0]);users.append(uid)
            code,login=req('/api/login',{'username':username,'password':c.env['SEED_TEACHER_PASSWORD' if role=='teacher' else 'SEED_STUDENT_PASSWORD']},who=None);assert code==200;c.tokens['flow_'+role]=login['token']
        boundary='flow'+uuid.uuid4().hex
        content='分数模型用相同的整体讨论每一份，先读条件，再画图比较，说明整体与部分的关系，检查单位是否一致。'
        fields=[('title',prefix+'分数讲义'),('subject','math')]
        raw=''.join(f'--{boundary}\r\nContent-Disposition: form-data; name="{k}"\r\n\r\n{v}\r\n' for k,v in fields)+f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="flow.md"\r\nContent-Type: text/markdown\r\n\r\n{content}\r\n--{boundary}--\r\n'
        request=urllib.request.Request(c.base+'/api/materials',data=raw.encode(),headers={'Authorization':'Bearer '+c.tokens['flow_teacher'],'Content-Type':'multipart/form-data; boundary='+boundary})
        with urllib.request.urlopen(request,timeout=30) as r:assert r.status==201;material=json.load(r)['id']
        for _ in range(100):
            if c.sql(f'SELECT COUNT(*) FROM knowledge_chunks WHERE material_id={material}')[0][0]!='0':break
            time.sleep(.1)
        code,p=req('/api/knowledge-points',{'subject':'math','name':prefix+'分数','chapter':'分数','description':'整体与部分关系','material_ids':[material],'assignment_ids':[]});assert code==201;point=p['id'];point_ids.append(point);assert student(f'/api/knowledge-points/{point}/favorite',method='PUT')[0]==204
        c.control(correct=False)
        code,v=req('/api/lesson-plans/generate',{'subject':'math','chapter':'分数','objectives':'结合整体说明部分的意义','periods':1,'minutes_per_period':45,'material_ids':[material],'client_key':prefix});assert code==200 and v['draft'];lesson=v['draft']['id'];assert all(x['material_id']==material for x in v['draft']['citations'])
        assert student('/api/lesson-plans/'+str(lesson))[0]==403
        questions=[{'type':'short_answer','stem':prefix+'分数思路'+str(i),'options':[],'answer':['参考'+str(i)+'整体关系'],'explanation':'从同一整体理解各部分[1]','knowledge_point_ids':[point],'difficulty':'medium','citation_indices':[1]} for i in range(2)]
        c.control(correct=False,bank_response={'questions':questions});code,v=req('/api/question-drafts/generate',{'subject':'math','material_ids':[material],'knowledge_point_ids':[point],'question_types':['short_answer'],'difficulty':'medium','question_count':2,'client_key':prefix});assert code==200 and v['draft'];draft=v['draft']['id'];code,v=req(f'/api/question-drafts/{draft}/approve',{'expected_version':1});assert code==200
        code,v=req('/api/paper-drafts',{'subject':'math','title':prefix,'items':[{'question_id':id,'question_version':1,'score':10} for id in v['bank_ids']],'client_key':prefix});assert code==201;paper=v['id'];code,v=req(f'/api/paper-drafts/{paper}/publish',{'expected_version':1});assert code==201;published=v['id'];children=[q['assignment_id'] for q in v['questions']]
        answers=[{'assignment_id':id,'answer':'先画出整体，再比较每一份的关系。'} for id in children]
        code,attempt=c.request(f'/api/published-papers/{published}/attempts',{'answers':answers},who='flow_student',headers={'Idempotency-Key':prefix})[:2];assert code==202
        for item in attempt['items']:assert wait(item['task']['task_id'])['result']['correct'] is False
        stats=student('/api/learning-diagnosis?subject=math')[1];assert stats['samples']==0
        code,v=student('/api/walkthroughs',{'assignment_id':children[0],'client_key':prefix});assert code==201;walk=v['id']
        for stage in ['read','method','derive','check']:
            for action in ['discuss','finish' if stage=='check' else 'advance']:
                v=student('/api/walkthroughs/'+str(walk))[1];code,stream=student(f'/api/walkthroughs/{walk}/turns',{'content':'先比较同一整体，说明图示的含义并检查条件。','action':action,'expected_version':v['version'],'client_key':prefix+'-'+stage+'-'+action});assert code==200 and 'event: done' in stream
        assert student('/api/walkthroughs/'+str(walk))[1]['stage']=='completed'
        for i,item in enumerate(attempt['items']):assert req(f'/api/submissions/{item["id"]}/review',{'score':10 if i==0 else 4,'correct':i==0,'expected_version':0,'error_tags':[] if i==0 else ['concept']},method='PUT')[0]==200
        assert student('/api/paper-attempts/'+str(attempt['id']))[1]['final_score']==14
        code,g=student(f'/api/submissions/{attempt["items"][1]["id"]}/explanation',{});assert code==200 and g['citations'] and all(x['material_id']==material for x in g['citations'])
        stats=student('/api/learning-diagnosis?subject=math')[1];assert stats['samples']==2 and stats['score']==14 and stats['score_percent']==70
        code,v=student('/api/learning-diagnosis/explanation',{'subject':'math','days':30,'fingerprint':stats['fingerprint']});assert code==200 and v['explanation']
        c.control(graph_response={'nodes':[{'name':prefix+'关联概念','chapter':'分数','description':'联系整体与部分'}],'edges':[{'from':p['name'],'to':prefix+'关联概念','kind':'related'}],'citation_indices':[1]})
        code,v=req('/api/knowledge-graph/drafts/generate',{'subject':'math','material_ids':[material],'client_key':prefix});assert code==200 and v['draft'];graphdraft=v['draft']['id'];code,g=req(f'/api/knowledge-graph/drafts/{graphdraft}/approve',{'expected_version':1});assert code==200;point_ids.extend(g['point_ids'])
        node=next(n for n in student('/api/knowledge-graph?subject=math')[1]['nodes'] if n['id']==point);assert node['reviewed']==2 and node['incorrect']==1 and node['weak'] is True
        assert student('/api/review-cards/sync',{})[0]==200;cards=student('/api/review-cards?subject=math&status=all')[1]['items'];assert len(cards)==2
        mistake=next(v for v in cards if v['source_kind']=='mistake');feedback={'feedback':'good','expected_version':mistake['version'],'client_key':prefix};code,done=student(f'/api/review-cards/{mistake["id"]}/complete',feedback);assert code==200 and done['level']==1;assert student(f'/api/review-cards/{mistake["id"]}/complete',feedback)[1]==done
        item=student('/api/paper-attempts/'+str(attempt['id']))[1]['items'][1];assert req(f'/api/submissions/{item["id"]}/review',{'score':10,'correct':True,'expected_version':item['task']['review_version']},method='PUT')[0]==200
        assert not any(v['source_kind']=='mistake' for v in student('/api/review-cards?subject=math&status=all')[1]['items']);assert len(student('/api/review-completions?subject=math')[1]['items'])==1
        assert student('/api/paper-attempts/'+str(attempt['id']))[1]['final_score']==20
        # Original single-question path remains independently usable.
        code,legacy=req('/api/assignments',{'subject':'math','title':prefix+'单题','question':'说明整体的含义','expected_answer':'合成参考','max_score':10});assert code==201;children.append(legacy['id'])
        code,r=c.request(f'/api/assignments/{legacy["id"]}/submissions',{'answer':'整体表示比较的统一对象'},who='flow_student',headers={'Idempotency-Key':prefix})[:2];assert code==202;wait(r['task_id'])
        return {'six_extensions':'lesson/bank/walk/diagnosis/graph/review connected on same scoped data','grading':'AI zero formal samples; final14 then20; single and multi preserved','citations':'own authorized lecture','review':'idempotent, corrected source removed, history retained','model':'local deterministic gateway; live tools evaluation separate'}
    finally:
        c.control()
        if walk:c.sql('DELETE FROM walkthroughs WHERE id='+str(walk))
        if graphdraft:c.sql('DELETE FROM knowledge_graph_drafts WHERE id='+str(graphdraft))
        if published:c.sql('DELETE FROM published_papers WHERE id='+str(published))
        for id in children:c.sql(f'DELETE FROM submissions WHERE assignment_id={id};DELETE FROM grade_assignments WHERE id={id}')
        if paper:c.sql('DELETE FROM paper_drafts WHERE id='+str(paper))
        if draft:c.sql('DELETE FROM bank_questions WHERE origin_draft_id='+str(draft)+';DELETE FROM question_drafts WHERE id='+str(draft))
        if lesson:c.sql('DELETE FROM lesson_plans WHERE id='+str(lesson))
        for id in set(point_ids):c.sql('DELETE FROM knowledge_points WHERE id='+str(id))
        if material:
            detail=req('/api/materials/'+str(material))[1];assert req('/api/materials/'+str(material),{'version':detail['version']},method='DELETE')[0]==204
        for uid in reversed(users):c.sql(f'DELETE FROM sessions WHERE user_id={uid};DELETE FROM users WHERE id={uid}')
        c.tokens.pop('flow_teacher',None);c.tokens.pop('flow_student',None)
        assert before==c.sql('CHECKSUM TABLE '+tables),'Original business rows must be preserved after fixture cleanup'
