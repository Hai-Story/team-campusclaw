"""L1-L3: private drafts, evidence, retries/cancel and actual database side effects."""
from concurrent.futures import ThreadPoolExecutor
import http.client,json,time,uuid

def run(c):
    prefix=c.run_id+'-lesson-';teacher2=None;second_token=None;empty_id=None
    def gate(path,body=None):
        script="const r=await fetch('http://127.0.0.1:8089'+process.argv[1],process.argv[2]?{method:'POST',headers:{'Content-Type':'application/json'},body:process.argv[2]}:{});process.stdout.write(await r.text())"
        return json.loads(c.compose('exec','-T','gateway','node','--input-type=module','-e',script,path,'' if body is None else json.dumps(body)))
    def req(path,body=None,who='teacher',method=None):
        if who=='teacher2':return c.request(path,body,who=None,method=method,headers={'Authorization':'Bearer '+second_token})[:2]
        return c.request(path,body,who,method)[:2]
    def generate(**changes):
        return req('/api/lesson-plans/generate',{**valid,'client_key':prefix+uuid.uuid4().hex[:12],**changes})
    def count():return int(c.sql("SELECT COUNT(*) FROM lesson_plans WHERE client_key LIKE '"+prefix+"%'")[0][0])
    def wait_calls():
        for _ in range(40):
            if gate('/stats')['calls']>0:return
            time.sleep(.05)
        raise AssertionError('model call did not start')
    try:
        c.token('teacher');c.token('student');c.token('other')
        materials=req('/api/materials')[1]
        mid=next((m['id'] for m in materials if '数学' in m['title'] or '分数' in m['title']),materials[0]['id'])
        foreign=req('/api/materials',who='other')[1][0]['id']
        valid={'subject':'math','chapter':'分数','objectives':'结合本班讲义理解分数的意义','periods':1,'minutes_per_period':45,'material_ids':[mid],'client_key':prefix+'same'}
        c.control()
        assert req('/api/lesson-plans',who=None)[0]==401
        assert req('/api/lesson-plans/generate',valid,who='student')[0]==403
        for extra in [{'subject':'all'},{'chapter':' '},{'objectives':''},{'periods':0},{'periods':9},{'minutes_per_period':9},{'material_ids':[]},{'material_ids':[mid,mid]},{'class_id':2},{'client_key':'bad key'}]:
            assert generate(**extra)[0]==400,extra
        assert generate(material_ids=[foreign])[0]==404
        assert generate(material_ids=[9223372036854775807])[0]==404
        assert gate('/stats')['calls']==0
        c.sql("INSERT INTO materials(class_id,title,file_path,uploaded_by) SELECT class_id,'apitest-empty-"+c.run_id+"','isolated-no-file',id FROM users WHERE username='teacher_a'")
        empty_id=int(c.sql("SELECT id FROM materials WHERE title='apitest-empty-"+c.run_id+"'")[0][0])
        code,nohit=generate(material_ids=[empty_id]);assert code==200 and nohit['status']=='insufficient_evidence' and nohit['draft'] is None
        assert gate('/stats')['calls']==0 and count()==0
        code,result=req('/api/lesson-plans/generate',valid);assert code==200,(code,result)
        p=result['draft'];pid=p['id'];path='/api/lesson-plans/'+str(pid)
        assert result['status']=='generated' and p['version']==1 and p['citations'] and all(x['material_id']==mid and x['available'] for x in p['citations'])
        assert '45分钟' in p['body'] and all(x in p['body'] for x in ['教学目标','教学重点','教学难点','课堂活动','资源清单'])
        calls=gate('/stats')['calls'];assert calls==1
        assert req('/api/lesson-plans/generate',valid)[1]['draft']['id']==pid and count()==1 and gate('/stats')['calls']==calls
        assert req('/api/lesson-plans/generate',{**valid,'objectives':'different'})[0]==409
        assert req(path,who='student')[0]==403 and req('/api/lesson-plans',who='student')[0]==403
        username='apitest_lecturer_'+c.run_id
        c.sql("INSERT INTO users(username,password_hash,role,class_id) SELECT '"+username+"',password_hash,'teacher',class_id FROM users WHERE username='teacher_a'")
        teacher2=int(c.sql("SELECT id FROM users WHERE username='"+username+"'")[0][0])
        code,login=req('/api/login',{'username':username,'password':c.env['SEED_TEACHER_PASSWORD']},who=None);assert code==200
        second_token=login['token']
        assert req(path,who='teacher2')==req('/api/lesson-plans/9223372036854775807',who='teacher2') and req(path,who='teacher2')[0]==404
        assert req('/api/lesson-plans',who='teacher2')[1]['items']==[]
        c.sql('UPDATE users SET class_id=(SELECT class_id FROM (SELECT class_id FROM users WHERE username=\'student_b1\') q) WHERE id='+str(teacher2))
        assert req(path,who='teacher2')[0]==404
        marker='private-lesson-marker-'+c.run_id
        update={'title':'私有教案-'+c.run_id,'body':p['body']+'\n'+marker,'expected_version':1}
        with ThreadPoolExecutor(max_workers=2) as pool:outcomes=list(pool.map(lambda _:req(path,update,method='PUT')[0],range(2)))
        assert sorted(outcomes)==[200,409],outcomes
        assert req(path)[1]['version']==2 and marker in req(path)[1]['body']
        assert req(path,{'expected_version':1},method='DELETE')[0]==409
        assert req('/api/search',{'query':marker,'mode':'keyword'},who='student')[0]==200
        assert not any(marker in x['excerpt'] for x in req('/api/search',{'query':marker,'mode':'keyword'},who='student')[1]['hits'])
        assert c.sql("SELECT COUNT(*) FROM knowledge_entries WHERE body_text LIKE '%"+marker+"%'")[0][0]=='0'
        bad={"goals":["目标"],"key_points":["重点"],"difficulties":["难点"],"activities":[{"title":"活动","minutes":44,"steps":["讨论"]}],"resources":["讲义"],"citation_indices":[1]}
        for override in [{'lesson_raw':'not json'},{'lesson_response':bad},{'lesson_response':{**bad,'activities':[{**bad['activities'][0],'minutes':45}],'citation_indices':[999]}}]:
            before=count();c.control(**override);assert generate()[0]==503 and count()==before
        c.control(statuses=[503,200]);assert generate()[0]==200 and gate('/stats')['calls']==2
        c.control(statuses=[401]);before=count();assert generate()[0]==503 and gate('/stats')['calls']==1 and count()==before
        c.control(hold_lesson=True)
        with ThreadPoolExecutor(max_workers=1) as pool:
            pending=pool.submit(generate);wait_calls();assert generate()[0]==429;gate('/release',{});assert pending.result()[0]==200
        ref=p['citations'][0];chunk=int(ref['chunk_id']);original=c.sql('SELECT HEX(chunk_text) FROM knowledge_chunks WHERE id='+str(chunk))[0][0]
        c.control(hold_lesson=True)
        try:
            with ThreadPoolExecutor(max_workers=1) as pool:
                before=count();pending=pool.submit(generate);wait_calls();c.sql("UPDATE knowledge_chunks SET chunk_text=CONCAT(chunk_text,' changed') WHERE id="+str(chunk));gate('/release',{});assert pending.result()[0]==409 and count()==before
            read=req(path)[1];assert any(not x['available'] and x['excerpt']=='' for x in read['citations'])
        finally:c.sql("UPDATE knowledge_chunks SET chunk_text=UNHEX('"+original+"') WHERE id="+str(chunk))
        assert all(x['available'] for x in req(path)[1]['citations'])
        c.control(hold_lesson=True);before=count();connection=http.client.HTTPConnection('127.0.0.1',18081,timeout=5)
        connection.request('POST','/api/lesson-plans/generate',json.dumps({**valid,'client_key':prefix+'cancel'}),{'Content-Type':'application/json','Authorization':'Bearer '+c.token('teacher')});wait_calls();connection.close()
        for _ in range(40):
            if gate('/stats')['cancelled']>0:break
            time.sleep(.05)
        else:raise AssertionError('cancel did not reach gateway')
        gate('/release',{});assert count()==before
        assert req(path,{'expected_version':2},method='DELETE')[0]==204 and req(path)[0]==404
        return {'private_drafts':'owner+class; student403; other-teacher404','no_evidence':'no model call/no draft','idempotency':'one call/one draft','version_conflict':sorted(outcomes),'bad_output':'no write','retry_calls':2,'cancel':'gateway cancelled/no draft','source_changed':'409/no draft','student_search':'no private content'}
    finally:
        c.control()
        c.sql("DELETE FROM lesson_plans WHERE client_key LIKE '"+prefix+"%'")
        if empty_id:c.sql('DELETE FROM materials WHERE id='+str(empty_id))
        if teacher2:
            if second_token:
                try:req('/api/logout',{},who='teacher2')
                except Exception:pass
            c.sql('DELETE FROM sessions WHERE user_id='+str(teacher2)+'; DELETE FROM users WHERE id='+str(teacher2))
