"""B1-B3: strict structured questions, private approval, rollback and paper snapshots."""
from concurrent.futures import ThreadPoolExecutor
import copy,http.client,json,time,uuid

def run(c):
    prefix='bank-'+c.run_id;point=None;foreign=None;empty=None;drafts=[];papers=[];teacher2=None;token2=None;trigger='apitest_bank_'+c.run_id;constraint=False
    def req(path,body=None,who='teacher',method=None):
        if who=='teacher2':return c.request(path,body,who=None,method=method,headers={'Authorization':'Bearer '+token2})[:2]
        return c.request(path,body,who,method,timeout=130 if path.endswith('/generate') else 10)[:2]
    def generate(key=None,**change):return req('/api/question-drafts/generate',{**base,'client_key':key or prefix+'-'+uuid.uuid4().hex[:8],**change})
    def gate():return c.fixture('/stats')
    def release():return c.fixture('/release',{})
    def waitcall():
        for _ in range(50):
            if gate()['calls']>0:return
            time.sleep(.05)
        raise AssertionError('bank model never started')
    try:
        c.control();c.token('teacher');c.token('student');c.token('peer');c.token('other')
        mid=req('/api/materials')[1][0]['id'];code,p=req('/api/knowledge-points',{'subject':'math','name':prefix+'知识点','chapter':'题库','description':'合成知识点','material_ids':[mid],'assignment_ids':[]});assert code==201;point=p['id']
        classid,userid=map(int,c.sql("SELECT class_id,id FROM users WHERE username='teacher_a'")[0]);c.sql("INSERT INTO knowledge_points(class_id,subject,name,description,created_by) SELECT class_id,'math','"+prefix+"-foreign','',id FROM users WHERE username='student_b1'");foreign=int(c.sql("SELECT id FROM knowledge_points WHERE name='"+prefix+"-foreign'")[0][0])
        base={'subject':'math','material_ids':[mid],'knowledge_point_ids':[point],'question_types':['single_choice','multiple_choice','true_false','short_answer'],'difficulty':'medium','question_count':4,'client_key':prefix+'-same'}
        assert req('/api/bank-questions',who=None)[0]==401
        for path in ['/api/question-drafts','/api/bank-questions','/api/paper-drafts']:assert req(path,who='student')[0]==403
        for delta in [{'subject':'all'},{'question_count':0},{'question_count':11},{'question_types':['bad']},{'question_types':['single_choice','single_choice']},{'difficulty':'impossible'},{'knowledge_point_ids':[]},{'class_id':2}]:assert generate(**delta)[0]==400,delta
        assert generate(knowledge_point_ids=[foreign])[0]==404
        assert gate()['calls']==0
        c.sql("INSERT INTO materials(class_id,title,file_path,uploaded_by) VALUES("+str(classid)+",'"+prefix+"-empty','isolated-no-file',"+str(userid)+')');empty=int(c.sql("SELECT id FROM materials WHERE title='"+prefix+"-empty'")[0][0]);code,nohit=generate(material_ids=[empty]);assert code==200 and nohit['draft'] is None and gate()['calls']==0
        code,result=generate(prefix+'-same');assert code==200 and result['draft'];d=result['draft'];drafts.append(d['id']);path='/api/question-drafts/'+str(d['id']);questions=d['questions'];assert len(questions)==4 and {q['type'] for q in questions}==set(base['question_types'])
        assert all(q['knowledge_point_ids']==[point] and q['citation_indices']==[1] for q in questions)
        calls=gate()['calls'];assert generate(prefix+'-same')[1]['draft']['id']==d['id'] and gate()['calls']==calls
        assert generate(prefix+'-same',difficulty='easy')[0]==409
        assert req(path,who='student')[0]==403
        username='bank_teacher_'+c.run_id;c.sql("INSERT INTO users(username,password_hash,role,class_id) SELECT '"+username+"',password_hash,'teacher',class_id FROM users WHERE username='teacher_a'");teacher2=int(c.sql("SELECT id FROM users WHERE username='"+username+"'")[0][0]);token2=req('/api/login',{'username':username,'password':c.env['SEED_TEACHER_PASSWORD']},who=None)[1]['token'];assert req(path,who='teacher2')==req('/api/question-drafts/9223372036854775807',who='teacher2')
        assert req('/api/search',{'query':questions[0]['stem'],'mode':'keyword'},who='student')[1]['hits']==[]
        assert c.sql("SELECT COUNT(*) FROM knowledge_entries WHERE body_text LIKE '%"+prefix+"%'")[0][0]=='0'
        oversized=copy.deepcopy(questions);oversized[0]['stem']='x'*140000;assert req(path,{'questions':oversized,'expected_version':1},method='PUT')[0]==413
        bad=copy.deepcopy(questions);bad[0]['answer']=['Z'];code,error=req(path,{'questions':bad,'expected_version':1},method='PUT');assert code==400 and '第1题' in error['error']
        bad=copy.deepcopy(questions);bad[1]['stem']=questions[0]['stem'];assert req(path,{'questions':bad,'expected_version':1},method='PUT')[0]==400
        bad=copy.deepcopy(questions);bad[0]['citation_indices']=[999];assert req(path,{'questions':bad,'expected_version':1},method='PUT')[0]==400
        # A second pending batch can race with the first review, but cannot duplicate the bank.
        c.control(bank_response={'questions':questions});code,other=generate(prefix+'-duplicate');assert code==200;dup=other['draft'];drafts.append(dup['id'])
        ref=d['citations'][0];chunk=ref['chunk_id'];original=c.sql('SELECT HEX(chunk_text) FROM knowledge_chunks WHERE id='+str(chunk))[0][0]
        try:
            c.sql("UPDATE knowledge_chunks SET chunk_text=CONCAT(chunk_text,' changed') WHERE id="+str(chunk));assert req(path+'/approve',{'expected_version':1})[0]==409
            assert c.sql('SELECT COUNT(*) FROM bank_questions WHERE origin_draft_id='+str(d['id']))[0][0]=='0' and req(path)[1]['status']=='pending'
        finally:c.sql("UPDATE knowledge_chunks SET chunk_text=UNHEX('"+original+"') WHERE id="+str(chunk))
        with ThreadPoolExecutor(max_workers=2) as pool:outcomes=list(pool.map(lambda _:req(path+'/approve',{'expected_version':1}),range(2)))
        assert [x[0] for x in outcomes]==[200,200] and outcomes[0][1]['bank_ids']==outcomes[1][1]['bank_ids'];ids=outcomes[0][1]['bank_ids'];assert len(ids)==4
        assert req('/api/question-drafts/'+str(dup['id'])+'/approve',{'expected_version':1})[0]==409
        assert c.sql('SELECT COUNT(*) FROM bank_questions WHERE origin_draft_id='+str(dup['id']))[0][0]=='0'
        assert generate(prefix+'-already-exists')[0]==409
        q0=req('/api/bank-questions/'+str(ids[0]))[1];q1=req('/api/bank-questions/'+str(ids[1]))[1]
        assert req('/api/bank-questions/'+str(ids[0]),who='teacher2')==req('/api/bank-questions/9223372036854775807',who='teacher2')
        selection=[{'question_id':q0['id'],'question_version':1,'score':10},{'question_id':q1['id'],'question_version':1,'score':20}]
        paperbody={'subject':'math','title':prefix+'试卷','items':selection,'client_key':prefix+'-paper'}
        code,paper=req('/api/paper-drafts',paperbody);assert code==201 and paper['total_score']==30;papers.append(paper['id']);pp='/api/paper-drafts/'+str(paper['id']);assert req('/api/paper-drafts',paperbody)[1]['id']==paper['id'];assert req('/api/paper-drafts',{**paperbody,'title':'different'})[0]==409
        assert req(pp,who='student')[0]==403 and req(pp,who='teacher2')==req('/api/paper-drafts/9223372036854775807',who='teacher2')
        for change in [{'title':''},{'items':[selection[0],selection[0]]},{'items':[{'question_id':ids[0],'question_version':0,'score':10}]},{'items':[{'question_id':i,'question_version':1,'score':100} for i in range(1,12)]}]:assert req('/api/paper-drafts',{**paperbody,**change,'client_key':prefix+'-bad-'+uuid.uuid4().hex[:4]})[0]==400,change
        assert req('/api/paper-drafts',{**paperbody,'subject':'physics','client_key':prefix+'-wrong-subject'})[0]==404
        revised={**q0['question'],'stem':q0['question']['stem']+' 修订'};update={'question':revised,'expected_version':1,'confirm_review':True}
        assert req('/api/bank-questions/'+str(q0['id']),{**update,'confirm_review':False},method='PUT')[0]==400
        with ThreadPoolExecutor(max_workers=2) as pool:codes=list(pool.map(lambda _:req('/api/bank-questions/'+str(q0['id']),update,method='PUT')[0],range(2)))
        assert sorted(codes)==[200,409];assert req(pp)[1]['items'][0]['question']['stem']==q0['question']['stem']
        reordered=[{**selection[1],'score':11},{**selection[0],'score':22}];code,v=req(pp,{'title':paperbody['title'],'items':reordered,'expected_version':1},method='PUT');assert code==200 and v['total_score']==33 and v['items'][1]['question']['stem']==q0['question']['stem']
        reordered[1]['question_version']=2;code,v=req(pp,{'title':paperbody['title'],'items':reordered,'expected_version':2},method='PUT');assert code==200 and v['items'][1]['question']['stem']==revised['stem']
        assert req('/api/bank-questions/'+str(q0['id']),{'expected_version':2},method='DELETE')[0]==204
        assert req(pp)[1]['items'][1]['question']['stem']==revised['stem'];assert req(pp,{'title':paperbody['title'],'items':reordered,'expected_version':3},method='PUT')[0]==200
        try:
            c.sql("UPDATE knowledge_chunks SET chunk_text=CONCAT(chunk_text,' retired') WHERE id="+str(chunk))
            for entry in [req('/api/bank-questions/'+str(q1['id']))[1],next(q for q in req('/api/bank-questions?page_size=100')[1]['items'] if q['id']==q1['id']),next(q for q in req('/api/question-drafts?page_size=100')[1]['items'] if q['id']==d['id'])]:
                assert any(not ref['available'] and ref['excerpt']=='' for ref in entry['citations'])
            assert all(any(not ref['available'] and ref['excerpt']=='' for ref in item['citations']) for item in req(pp)[1]['items'])
        finally:c.sql("UPDATE knowledge_chunks SET chunk_text=UNHEX('"+original+"') WHERE id="+str(chunk))
        # Force the second insert to fail; the first insert and review state must roll back.
        failure_questions=copy.deepcopy(questions)
        for i,q in enumerate(failure_questions):q['stem']=prefix+('-failure-second' if i==1 else '-failure-'+str(i))
        c.control(bank_response={'questions':failure_questions});code,failed=generate(prefix+'-rollback');assert code==200;fd=failed['draft'];drafts.append(fd['id'])
        c.sql('ALTER TABLE bank_questions ADD CONSTRAINT '+trigger+" CHECK(stem<>'"+prefix+"-failure-second')");constraint=True
        try:
            assert req('/api/question-drafts/'+str(fd['id'])+'/approve',{'expected_version':1})[0]==503
            assert c.sql('SELECT COUNT(*) FROM bank_questions WHERE origin_draft_id='+str(fd['id']))[0][0]=='0' and req('/api/question-drafts/'+str(fd['id']))[1]['status']=='pending'
        finally:c.sql('ALTER TABLE bank_questions DROP CHECK '+trigger);constraint=False
        for settings in [{'bank_raw':'bad JSON'},{'bank_response':{'questions':[questions[0]]}},{'bank_response':{'questions':[questions[0]]*4}}]:
            c.control(**settings);assert generate()[0]==503
        c.control(hold_bank=True)
        with ThreadPoolExecutor(max_workers=1) as pool:
            pending=pool.submit(generate,prefix+'-hold');waitcall();assert generate(prefix+'-parallel')[0]==429;release();code,v=pending.result();assert code==200;drafts.append(v['draft']['id'])
        c.control(hold_bank=True);conn=http.client.HTTPConnection('127.0.0.1',18081,timeout=5);conn.request('POST','/api/question-drafts/generate',json.dumps({**base,'client_key':prefix+'-cancel'}),{'Content-Type':'application/json','Authorization':'Bearer '+c.token('teacher')});waitcall();conn.close()
        for _ in range(40):
            if gate()['cancelled']>0:break
            time.sleep(.05)
        else:raise AssertionError('cancel did not reach bank model')
        release();assert c.sql("SELECT COUNT(*) FROM question_drafts WHERE client_key='"+prefix+"-cancel'")[0][0]=='0';c.control()
        before=c.sql('CHECKSUM TABLE question_drafts,bank_questions,paper_drafts,materials,submissions,grade_results,grade_reviews')
        for _ in range(2):
            c.compose('restart','api')
            for _ in range(60):
                try:
                    if req(pp)[0]==200:break
                except Exception:pass
                time.sleep(.1)
            assert c.sql('CHECKSUM TABLE question_drafts,bank_questions,paper_drafts,materials,submissions,grade_results,grade_reviews')==before
        timings=[c.request('/api/bank-questions',who='teacher')[2]*1000 for _ in range(100)];p95=sorted(timings)[94];assert p95<=500
        return {'types':4,'privacy':'student403; other teacher same404; no private search entries','approval':'source recheck, same-version replay, duplicate and late insert rollback','bank_edit':[200,409],'papers':'order/total; original snapshot retained; explicit version refresh; delete retained','cancel':'gateway cancelled/no draft','migration_restarts':2,'list_p95_ms':round(p95,2)}
    finally:
        c.control()
        if constraint:c.sql('ALTER TABLE bank_questions DROP CHECK '+trigger)
        c.sql("DELETE FROM bank_questions WHERE origin_draft_id IN (SELECT id FROM question_drafts WHERE client_key LIKE '"+prefix+"%'); DELETE FROM question_drafts WHERE client_key LIKE '"+prefix+"%'; DELETE FROM paper_drafts WHERE client_key LIKE '"+prefix+"%'")
        if point:c.sql('DELETE FROM knowledge_points WHERE id='+str(point))
        if foreign:c.sql('DELETE FROM knowledge_points WHERE id='+str(foreign))
        if empty:c.sql('DELETE FROM materials WHERE id='+str(empty))
        if teacher2:c.sql('DELETE FROM sessions WHERE user_id='+str(teacher2)+'; DELETE FROM users WHERE id='+str(teacher2))
