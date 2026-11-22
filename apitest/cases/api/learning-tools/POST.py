"""D4: real native MCP registration, approval, RPC scopes, fingerprints and read-only budgets."""
import json,time

def run(c):
    servers=[];teacher_b=None;point=None
    def rpc(name,args,who='student',method='tools/call'):
        return c.request('/api/learning-tools/mcp',{'jsonrpc':'2.0','id':1,'method':method,'params':{'name':name,'arguments':args} if method=='tools/call' else {}},who=who)[:2]
    def call(name,args,who='student'):
        code,value=rpc(name,args,who);assert code==200 and value['result']['isError'] is False,(code,value);return json.loads(value['result']['content'][0]['text'])
    def api(path,body=None,who='teacher',method=None):return c.request(path,body,who=who,method=method)[:2]
    def enable(who='teacher'):
        code,server=api('/api/mcp/learning-servers',{},who);assert code==200,(code,server);servers.append((server['id'],who));return server
    def approve(sid,who='teacher'):
        code,items=api(f'/api/mcp/servers/{sid}/tools',who=who);assert code==200
        for tool in items:
            assert api(f'/api/mcp/servers/{sid}/tools/{tool["name"]}/approve',{'fingerprint':tool['fingerprint']},who)[0]==200
        return items
    try:
        for who in ['teacher','student','peer','other']:c.token(who)
        assert api('/api/mcp/learning-servers',{},who=None)[0]==401
        assert api('/api/mcp/learning-servers',{},who='student')[0]==403
        for body in [None,[],{'class_id':2}]:
            if body is None:
                assert c.request('/api/mcp/learning-servers',None,who='teacher',method='POST')[0]==400
            else:assert api('/api/mcp/learning-servers',body)[0]==400
        # Existing remote discovery continues to use the approved network target.
        code,remote=api('/api/mcp/servers',{'name':'remote-'+c.run_id,'base_url':'http://mcp-demo:8090','auth_token':''});assert code==201;servers.append((remote['id'],'teacher'))
        assert api(f'/api/mcp/servers/{remote["id"]}/connect',{})[0]==200
        remote_tools=api(f'/api/mcp/servers/{remote["id"]}/tools')[1];assert {t['name'] for t in remote_tools}=={'formula_hint','unit_convert'}
        a=enable();sid=a['id'];assert a['base_url']=='builtin://learning' and not a['has_auth']
        again=api('/api/mcp/learning-servers',{})[1];assert again['id']==sid
        assert api(f'/api/mcp/servers/{sid}',{'name':'evil','base_url':'http://mcp-demo:8090','auth_token':''},method='PUT')[0]==400
        assert api(f'/api/mcp/servers/{sid}/connect',{})[0]==200
        items=api(f'/api/mcp/servers/{sid}/tools')[1];assert {t['name'] for t in items}>={'knowledge_points','assignment_stats'} and not any(t['approved'] for t in items)
        init=rpc('',{},method='initialize');assert init[0]==200 and init[1]['result']['protocolVersion']=='2025-03-26'
        assert rpc('',{},method='tools/list')[1]['result']['tools']==[]
        assert rpc('assignment_stats',{'subject':'math'})[1]['result']['isError'] is True
        approve(sid)
        assert len(rpc('',{},method='tools/list')[1]['result']['tools'])>=2
        c.control(correct=False);aid=c.assignment();s=c.submit(aid)[1];c.wait(s['task_id'])
        assert api(f'/api/submissions/{s["submission_id"]}/review',{'score':4,'correct':False,'expected_version':0,'error_tags':['concept']},method='PUT')[0]==200
        code,p=api('/api/knowledge-points',{'subject':'math','name':'native-'+c.run_id,'chapter':'工具章节','description':'合成只读工具验证','material_ids':[],'assignment_ids':[aid]});assert code==201;point=p['id']
        c.control();counts=c.sql('SELECT COUNT(*) FROM knowledge_points;SELECT COUNT(*) FROM grade_reviews;SELECT COUNT(*) FROM submissions;')
        query=call('knowledge_points',{'subject':'math','q':'native-'+c.run_id});assert [v['id'] for v in query['items']]==[point]
        regular=api('/api/learning-diagnosis?subject=math',who='student')[1];tool=call('assignment_stats',{'subject':'math'});assert tool['source']=='teacher_final' and tool['totals']=={k:regular[k] for k in ['samples','correct','score','max_score','score_percent','correct_percent']}
        assert 'students' not in tool and 'evidence' not in tool
        for args in [{'subject':'all'},{'subject':'math','class_id':2},{'subject':'math','student_id':1},{'subject':'math','role':'teacher'},{'subject':'math','days':181}]:assert rpc('assignment_stats',args)[1]['result']['isError'] is True,args
        assert rpc('unknown_tool',{'subject':'math'})[1]['result']['isError'] is True
        c.sql(f"UPDATE mcp_tools SET fingerprint=REPEAT('0',64) WHERE server_id={sid} AND name='assignment_stats'")
        assert rpc('assignment_stats',{'subject':'math'})[1]['result']['isError'] is True
        assert api(f'/api/mcp/servers/{sid}/connect',{})[0]==200
        assert call('assignment_stats',{'subject':'math'})['totals']==tool['totals']
        for body in [{'jsonrpc':'2.0','id':{},'method':'tools/list'},{'jsonrpc':'1.0','id':1,'method':'tools/list'},{'jsonrpc':'2.0','id':1,'method':'tools/list','role':'teacher'}]:assert c.request('/api/learning-tools/mcp',body)[0]==400
        assert rpc('x'*200,{'subject':'math'})[1].get('error')
        # Enable a distinct B-class service; its callers can still only query B-class data.
        username='apitest_native_b_'+c.run_id
        c.sql("INSERT INTO users(username,password_hash,role,class_id) SELECT '"+username+"',a.password_hash,'teacher',b.class_id FROM users a JOIN users b ON b.username='student_b1' WHERE a.username='teacher_a'")
        teacher_b=int(c.sql("SELECT id FROM users WHERE username='"+username+"'")[0][0]);code,login=api('/api/login',{'username':username,'password':c.env['SEED_TEACHER_PASSWORD']},who=None);assert code==200;c.tokens['native_teacher_b']=login['token']
        unknown=api('/api/mcp/servers/999999999',who='native_teacher_b');assert api(f'/api/mcp/servers/{sid}',who='native_teacher_b')==unknown and unknown[0]==404
        b=enable('native_teacher_b');assert api(f'/api/mcp/servers/{b["id"]}/connect',{},'native_teacher_b')[0]==200;approve(b['id'],'native_teacher_b')
        assert call('knowledge_points',{'subject':'math','q':'native-'+c.run_id},who='other')['items']==[]
        bstats=api('/api/learning-diagnosis?subject=math',who='other')[1];assert call('assignment_stats',{'subject':'math'},who='other')['totals']['samples']==bstats['samples']
        start=time.monotonic();codes=[rpc('knowledge_points',{'subject':'math'},who='peer')[0] for _ in range(31)];assert time.monotonic()-start<60 and codes[:30]==[200]*30 and codes[-1]==429,codes
        assert call('assignment_stats',{'subject':'math'})['totals']==tool['totals']
        assert c.fixture('/stats')['calls']==0,'Read-only tools must not invoke a model'
        assert counts==c.sql('SELECT COUNT(*) FROM knowledge_points;SELECT COUNT(*) FROM grade_reviews;SELECT COUNT(*) FROM submissions;')
        assert api(f'/api/mcp/servers/{sid}/disconnect',{})[0]==200 and rpc('assignment_stats',{'subject':'math'})[0]==404
        assert api(f'/api/mcp/servers/{sid}/connect',{})[0]==200 and call('assignment_stats',{'subject':'math'})['source']=='teacher_final'
        return {'discovery':'native and existing remote','approval':'required; current fingerprint enforced','tools':'same scoped business data; no model calls','scope':'A/B class and own student; identity injection denied','budget':'30/min; actor-isolated','business_writes':0}
    finally:
        if point:c.sql('DELETE FROM knowledge_points WHERE id='+str(point))
        for sid,who in reversed(servers):api('/api/mcp/servers/'+str(sid),who=who,method='DELETE')
        if teacher_b:
            c.request('/api/logout',{},who='native_teacher_b');c.sql('DELETE FROM users WHERE id='+str(teacher_b))
