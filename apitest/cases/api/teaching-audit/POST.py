"""H2: ordinary teaching audit, actual usage, absent usage, failures and cancellation."""
import json,time,uuid,http.client

def run(c):
    prefix='audit-'+c.run_id;agents=[];skills=[];conversations=[];server=None
    def req(path,body=None,who='teacher',method=None):return c.request(path,body,who=who,method=method)[:2]
    def logs():return c.compose('logs','--no-color','--no-log-prefix','api')
    def audit(rid):
        for _ in range(80):
            entries=[]
            for line in logs().splitlines():
                try:v=json.loads(line)
                except ValueError:continue
                if v.get('request_id')==rid and v.get('event')=='generation':entries.append(v)
            if entries:
                assert len(entries)==1
                for _ in range(100):
                    code,stored,_,_=c.request('/api/audit-events?request_id='+rid,who='teacher')
                    if code==200 and stored['items'] and stored['items'][0]['result']!='started':break
                    time.sleep(.02)
                assert code==200 and stored['items'], 'Missing durable audit'
                row=stored['items'][0];assert row['model']['usage']==entries[0]['usage'] and row['skill_id']==entries[0]['skill_id']
                assert row['result']==({'complete':'succeeded'}.get(entries[0]['status'],entries[0]['status'])),row
                return entries[0]
            time.sleep(.05)
        raise AssertionError('Missing request-bound teaching audit')
    def turn(content):
        code,v=req('/api/conversations',{'agent_id':agents[-1]},who='student');assert code==201;conversations.append(v['id'])
        code,stream,_,headers=c.request(f'/api/conversations/{v["id"]}/messages',{'content':content,'capture_trace':False},timeout=130)
        rid=next(v for k,v in headers.items() if k.lower()=='x-request-id');assert code==200;return v['id'],stream,audit(rid)
    def skill(tools=''):
        code,v=req('/api/skills',{'source_text':'名称：审计示例-'+prefix+'-'+str(len(skills))+'\n适用条件：审计验证\n资料：\n工具：'+tools+'\n启用：是\n\n仅提供授权学习提示。'});assert code==201;skills.append(v['id'])
        code,a=req('/api/agents',{'name':prefix+'-'+str(len(skills)),'prompt':'给出学习思路。','enabled':True,'skill_ids':[v['id']],'material_ids':[]});assert code==201;agents.append(a['id']);return v['id']
    try:
        c.control();sid=skill();uid=int(c.sql("SELECT id FROM users WHERE username='student_a1'")[0][0])
        cid,stream,a=turn('审计验证 '+prefix);assert 'event: done' in stream and a['actor_id']==uid and a['skill_id']==sid and a['status']=='complete'
        assert a['calls']==1 and a['usage']['total_tokens']==32 and a['usage']['complete'] is True,a
        history=req('/api/conversations/'+str(cid),who='student')[1];assert not any(v.get('has_trace') for v in history['messages'])
        _,_,a=turn('普通学习问题 '+prefix);assert a['skill_id'] is None and a['status']=='complete'
        c.control(omit_stream_usage=True);_,stream,a=turn('审计验证 '+prefix);assert 'event: done' in stream and a['usage']['reported'] is False and a['usage']['missing_calls']==1 and a['usage']['complete'] is False
        c.control(statuses=[503]);_,stream,a=turn('审计验证 '+prefix);assert 'event: error' in stream and a['status']=='failed' and a['call_states']['failed']==1 and a['usage']['reported'] is False
        c.control(delay_ms=3000);code,v=req('/api/conversations',{'agent_id':agents[-1]},who='student');assert code==201;conversations.append(v['id'])
        conn=http.client.HTTPConnection('127.0.0.1',18081,timeout=10);conn.request('POST',f'/api/conversations/{v["id"]}/messages',json.dumps({'content':'审计验证 '+prefix}),{'Content-Type':'application/json','Authorization':'Bearer '+c.token('student')});response=conn.getresponse();rid=response.getheader('X-Request-ID');response.read(1)
        for _ in range(40):
            if c.fixture('/stats')['calls']>0:break
            time.sleep(.05)
        response.close();conn.close();a=audit(rid);assert a['status']=='cancelled',a
        # Native tools must contribute planning usage as well as the final stream.
        c.control();code,v=req('/api/mcp/learning-servers',{});assert code==200;server=v['id'];assert req(f'/api/mcp/servers/{server}/connect',{})[0]==200
        for t in req(f'/api/mcp/servers/{server}/tools')[1]:assert req(f'/api/mcp/servers/{server}/tools/{t["name"]}/approve',{'fingerprint':t['fingerprint']})[0]==200
        skill('knowledge_points@learning');c.control(tool_request='knowledge_points',tool_args={'subject':'math'});_,stream,a=turn('审计验证 '+prefix);assert 'event: done' in stream and a['calls']==3 and a['usage']['total_tokens']==96 and a['usage']['complete'] is True,a
        raw=logs();assert prefix not in raw and c.token('student') not in raw,'Sensitive content leaked into logs'
        return {'normal':'actor/skill/model/request bound; trace off','usage':'planning+stream actual totals; missing explicitly marked','failure':'failed/cancelled retain audit; never complete','sensitive':'body/token markers absent'}
    finally:
        c.control()
        for cid in conversations:req('/api/conversations/'+str(cid),who='student',method='DELETE')
        for id in reversed(agents):req('/api/agents/'+str(id),method='DELETE')
        for id in reversed(skills):req('/api/skills/'+str(id),method='DELETE')
        if server:req('/api/mcp/servers/'+str(server),method='DELETE')
