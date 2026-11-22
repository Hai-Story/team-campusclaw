"""M1-M4: immutable publication, atomic four-kind attempts and shared final-review consumers."""
import copy,json,time,uuid,hashlib
from concurrent.futures import ThreadPoolExecutor

def run(c):
    prefix='published-'+c.run_id;point=draft=paper=None;published=[];children=[];extra_papers=[];checks=[];chunk=None;original=None
    def req(path,body=None,who='teacher',method=None,**kw):return c.request(path,body,who,method,**kw)[:2]
    def submit(pid,answers,key=None,who='student'):return req(f'/api/published-papers/{pid}/attempts',{'answers':answers},who=who,headers={'Idempotency-Key':key or uuid.uuid4().hex})
    def state():return c.sql('SELECT COUNT(*) FROM paper_attempts;SELECT COUNT(*) FROM submissions;SELECT COUNT(*) FROM grade_tasks;SELECT COUNT(*) FROM grade_outbox;SELECT COUNT(*) FROM paper_attempt_submissions;')
    def review(item,correct,score):return req(f'/api/submissions/{item["id"]}/review',{'score':score,'correct':correct,'expected_version':item['task']['review_version'],'error_tags':[] if correct else ['concept']},method='PUT')
    try:
        c.control(correct=False);c.token('student');c.token('teacher');c.token('other');c.token('peer')
        mid=req('/api/materials')[1][0]['id'];code,p=req('/api/knowledge-points',{'subject':'math','name':prefix,'chapter':'多题','description':'合成知识点','material_ids':[mid],'assignment_ids':[]});assert code==201;point=p['id']
        kinds=['single_choice','multiple_choice','true_false','short_answer'];items=[]
        for i,kind in enumerate(kinds):items.append({'type':kind,'stem':prefix+' 第'+str(i+1)+'题','options':['甲','乙','丙'] if i<2 else [],'answer':['A'] if i==0 else ['A','C'] if i==1 else ['true'] if i==2 else ['参考内容'],'explanation':'合成说明[1]','knowledge_point_ids':[point],'difficulty':'medium','citation_indices':[1]})
        c.control(correct=False,bank_response={'questions':items});code,result=req('/api/question-drafts/generate',{'subject':'math','material_ids':[mid],'knowledge_point_ids':[point],'question_types':kinds,'difficulty':'medium','question_count':4,'client_key':prefix},timeout=130);assert code==200,(code,result);draft=result['draft']['id'];assert req(f'/api/question-drafts/{draft}/approve',{'expected_version':1})[0]==200
        ids=req(f'/api/question-drafts/{draft}')[1]['bank_ids'];code,p=req('/api/paper-drafts',{'subject':'math','title':prefix,'items':[{'question_id':id,'question_version':1,'score':10} for id in ids],'client_key':prefix});assert code==201;paper=p['id'];path=f'/api/paper-drafts/{paper}/publish'
        assert req(path,{'expected_version':1},who='student')[0]==403
        assert req(path,{'expected_version':0})[0]==400
        assert req(path,{'expected_version':2})[0]==409
        ref=result['draft']['citations'][0];chunk=ref['chunk_id'];original=c.sql('SELECT HEX(chunk_text) FROM knowledge_chunks WHERE id='+str(chunk))[0][0]
        c.sql("UPDATE knowledge_chunks SET chunk_text=CONCAT(chunk_text,'changed') WHERE id="+str(chunk));assert req(path,{'expected_version':1})[0]==409;c.sql("UPDATE knowledge_chunks SET chunk_text=CONVERT(UNHEX('"+original+"') USING utf8mb4) WHERE id="+str(chunk));original=None
        name='pub_reject_'+c.run_id;c.sql(f'ALTER TABLE published_paper_questions ADD CONSTRAINT {name} CHECK(position<2)');checks.append(('published_paper_questions',name));before=c.sql('SELECT COUNT(*) FROM published_papers;SELECT COUNT(*) FROM grade_assignments;')
        assert req(path,{'expected_version':1})[0]==503;assert before==c.sql('SELECT COUNT(*) FROM published_papers;SELECT COUNT(*) FROM grade_assignments;');c.sql(f'ALTER TABLE published_paper_questions DROP CHECK {name}');checks=[]
        with ThreadPoolExecutor(max_workers=2) as pool:r=list(pool.map(lambda _:req(path,{'expected_version':1}),range(2)))
        assert [v[0] for v in r]==[201,201] and r[0][1]==r[1][1],r
        pub=r[0][1];pid=pub['id'];published.append(pid);children=[q['assignment_id'] for q in pub['questions']];assert len(children)==4 and pub['total_score']==40
        student=req(f'/api/published-papers/{pid}',who='student')[1];assert all(not any(k in q for k in ['answer','explanation','citations']) for q in student['questions'])
        missing=req('/api/published-papers/999999999',who='other');assert missing[0]==404 and req(f'/api/published-papers/{pid}',who='other')==missing
        assert not set(children)&{a['id'] for a in req('/api/assignments')[1]['items']}
        assert req('/api/assignments/'+str(children[0]),who='student')[1]['paper_id']==pid
        assert c.request('/api/assignments/'+str(children[0])+'/submissions',{'answer':'A'},headers={'Idempotency-Key':'direct'})[0]==409
        # Published snapshot remains stable after source bank edits/deletes and private draft changes.
        edited=copy.deepcopy(items[0]);edited['stem']+='修改';assert req('/api/bank-questions/'+str(ids[0]),{'question':edited,'expected_version':1,'confirm_review':True},method='PUT')[0]==200
        assert req('/api/bank-questions/'+str(ids[1]),{'expected_version':1},method='DELETE')[0]==204
        assert req('/api/paper-drafts/'+str(paper),{'title':prefix+'新版','items':[{'question_id':id,'question_version':1,'score':10} for id in ids],'expected_version':1},method='PUT')[0]==200
        assert req(path,{'expected_version':1})[1]==pub and req(f'/api/published-papers/{pid}',who='student')[1]==student
        answers=[{'assignment_id':id,'answer':a} for id,a in zip(children,['A','C,A','true',' 合成作答 '])];before=state()
        for invalid in [[],answers[:-1],answers+[answers[0]],[answers[0],answers[0],*answers[2:]],[{**a,'answer':'Z'} if i==0 else a for i,a in enumerate(answers)],[{**a,'assignment_id':999999} if i==0 else a for i,a in enumerate(answers)]]:
            assert submit(pid,invalid)[0]==400,invalid
        assert before==state();assert submit(pid,answers,who='teacher')[0]==403;assert submit(pid,answers,who='other')[0]==404
        assert submit(pid,[{**a,'answer':'x'*1100000} if i==3 else a for i,a in enumerate(answers)])[0]==413
        name='attempt_reject_'+c.run_id;c.sql(f'ALTER TABLE paper_attempt_submissions ADD CONSTRAINT {name} CHECK(assignment_id<>{children[1]})');checks.append(('paper_attempt_submissions',name));assert submit(pid,answers)[0]==503;assert before==state();c.sql(f'ALTER TABLE paper_attempt_submissions DROP CHECK {name}');checks=[]
        key=prefix+'-same'
        with ThreadPoolExecutor(max_workers=2) as pool:r=list(pool.map(lambda _:submit(pid,answers,key),range(2)))
        assert [v[0] for v in r]==[202,202] and r[0][1]['id']==r[1][1]['id'] and [v['id'] for v in r[0][1]['items']]==[v['id'] for v in r[1][1]['items']],r
        attempt=r[0][1];assert attempt['final_score'] is None and len(attempt['items'])==4
        assert submit(pid,[{**a,'answer':'A,C'} if i==1 else a for i,a in enumerate(answers)],key)[1]['id']==attempt['id']
        assert submit(pid,[{**a,'answer':'B'} if i==0 else a for i,a in enumerate(answers)],key)[0]==409
        for item in attempt['items']:assert c.wait(item['task']['task_id'])['result']['correct'] is False
        detail=f'/api/paper-attempts/{attempt["id"]}';assert req(detail,who='peer')==req('/api/paper-attempts/999999999',who='peer')
        current=req(detail)[1];assert current['reviewed_count']==0 and current['final_score'] is None
        for i,item in enumerate(current['items']):
            assert review(item,i!=1,0 if i==1 else 10)[0]==200
            now=req(detail)[1];assert now['reviewed_count']==i+1
            if i<3:assert now['final_score'] is None
        now=req(detail)[1];assert now['final_score']==30 and now['reviewed_score']==30
        mistake_ids={x['id'] for x in req('/api/mistakes?subject=math',who='student')[1]['items']};assert current['items'][1]['id'] in mistake_ids and not ({v['id'] for i,v in enumerate(current['items']) if i!=1}&mistake_ids)
        stats=req('/api/learning-diagnosis?subject=math',who='student')[1];kp=next(p for p in stats['points'] if p['id']==point);assert kp['samples']==4 and kp['score']==30
        graph=req('/api/knowledge-graph?subject=math',who='student')[1];node=next(n for n in graph['nodes'] if n['id']==point);assert node['reviewed']==4
        assert req('/api/review-cards/sync',{},who='student')[0]==200;cards=req('/api/review-cards?subject=math&status=all&page_size=100',who='student')[1]['items'];assert any(v['source_id']==current['items'][1]['id'] and v['source_kind']=='mistake' for v in cards)
        assert review(now['items'][1],True,10)[0]==200;assert req(detail)[1]['final_score']==40
        newer=submit(pid,answers)[1];assert newer['id']!=attempt['id'] and newer['final_score'] is None
        stats=req('/api/learning-diagnosis?subject=math',who='student')[1];assert not any(p['id']==point and p['samples']>0 for p in stats['points'])
        assert req(detail)[1]['latest'] is False and len(req(f'/api/published-papers/{pid}/attempts')[1]['items'])==1
        for item in newer['items']:c.wait(item['task']['task_id'])
        with ThreadPoolExecutor(max_workers=2) as pool:twins=list(pool.map(lambda _:submit(pid,answers),range(2)))
        assert [v[0] for v in twins]==[202,202] and twins[0][1]['id']!=twins[1][1]['id']
        twins=[req('/api/paper-attempts/'+str(v[1]['id']))[1] for v in twins]
        assert sum(v['latest'] for v in twins)==1 and all(all(item['latest']==v['latest'] for item in v['items']) for v in twins)
        for v in twins:
            for item in v['items']:c.wait(item['task']['task_id'])
        # Large valid batch: 50 independent synthetic bank fixtures, 600KB of actual answers.
        scale=[]
        for i in range(50):
            item=copy.deepcopy(items[3]);item['stem']=prefix+'-scale-'+str(i);raw=json.dumps(item,ensure_ascii=False,separators=(',',':')).encode().hex();stem=item['stem'];digest=hashlib.sha256(stem.lower().encode()).hexdigest()
            c.sql(f"INSERT INTO bank_questions(class_id,created_by,subject,origin_draft_id,question_json,citations_json,stem,stem_hash) SELECT class_id,created_by,subject,origin_draft_id,CONVERT(UNHEX('{raw}') USING utf8mb4),citations_json,'{stem}','{digest}' FROM bank_questions WHERE id={ids[3]}")
            qid=int(c.sql(f"SELECT id FROM bank_questions WHERE origin_draft_id={draft} AND stem_hash='{digest}'")[0][0]);scale.append({'question_id':qid,'question_version':1,'score':20})
        assert req('/api/paper-drafts',{'subject':'math','title':prefix+'-invalid','items':scale+[scale[0]],'client_key':prefix+'-51'})[0]==400
        assert req('/api/paper-drafts',{'subject':'math','title':prefix+'-invalid','items':[{**x,'score':21} for x in scale],'client_key':prefix+'-1050'})[0]==400
        code,sc=req('/api/paper-drafts',{'subject':'math','title':prefix+'-50','items':scale,'client_key':prefix+'-50'});assert code==201;extra_papers.append(sc['id']);code,sp=req('/api/paper-drafts/'+str(sc['id'])+'/publish',{'expected_version':1});assert code==201;published.append(sp['id']);children.extend(q['assignment_id'] for q in sp['questions'])
        code,sa=submit(sp['id'],[{'assignment_id':q['assignment_id'],'answer':'字'*4000} for q in sp['questions']]);assert code==202 and len(sa['items'])==50 and sa['total_score']==1000
        for item in sa['items']:c.wait(item['task']['task_id'])
        for _ in range(5):req('/api/published-papers')
        times=[c.request('/api/published-papers',who='teacher')[2]*1000 for _ in range(100)];p95=sorted(times)[94];assert p95<=500
        return {'publish':'immutable/idempotent/version/real evidence; atomic rollback','types':4,'batch':'whole transaction; replay canonical; concurrent same IDs; mid-failure no orphan rows','privacy':'students no answers; same404; managed single submit409','final':'null until complete; sum current heads; latest whole attempts','integration':'mistakes/diagnosis/graph/review share per-question results','list_p95_ms':round(p95,2)}
    finally:
        c.control()
        for table,name in checks:c.sql(f'ALTER TABLE {table} DROP CHECK {name}')
        if original:c.sql("UPDATE knowledge_chunks SET chunk_text=CONVERT(UNHEX('"+original+"') USING utf8mb4) WHERE id="+str(chunk))
        for id in published:c.sql('DELETE FROM published_papers WHERE id='+str(id))
        for id in children:c.sql('DELETE FROM submissions WHERE assignment_id='+str(id)+';DELETE FROM grade_assignments WHERE id='+str(id))
        for extra in extra_papers:c.sql('DELETE FROM paper_drafts WHERE id='+str(extra))
        if paper:c.sql('DELETE FROM paper_drafts WHERE id='+str(paper))
        if draft:c.sql('DELETE FROM bank_questions WHERE origin_draft_id='+str(draft)+';DELETE FROM question_drafts WHERE id='+str(draft))
        if point:c.sql('DELETE FROM knowledge_points WHERE id='+str(point))
