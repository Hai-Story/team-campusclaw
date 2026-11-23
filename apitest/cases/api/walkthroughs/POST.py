"""W1-W3: ownership, stage transitions, failure recovery and persisted side effects."""
import json,time,http.client,uuid
from concurrent.futures import ThreadPoolExecutor

def run(c):
    ids=[];aid=c.assignment();key='walk-'+c.run_id
    def req(path,body=None,who='student',method=None):return c.request(path,body,who,method)[:2]
    def create(**extra):return req('/api/walkthroughs',{'assignment_id':aid,'client_key':key,**extra})
    def gate():return c.fixture('/stats')
    def release():return c.fixture('/release',{})
    def started():
        for _ in range(40):
            if gate()['calls']>0:return
            time.sleep(.05)
        raise AssertionError('call never started')
    try:
        c.control();c.token('student');c.token('teacher');c.token('peer');c.token('other')
        assert req('/api/walkthroughs',who='teacher')[0]==403 and req('/api/walkthroughs',who=None)[0]==401
        assert create(assignment_id=0)[0]==400 and create(class_id=99)[0]==400
        assert create(assignment_id=9223372036854775807)[0]==404
        code,v=create();assert code==201 and v['stage']=='read' and v['turns']==[];ids.append(v['id']);wid=v['id'];path='/api/walkthroughs/'+str(wid);turn=path+'/turns'
        assert 'reference' not in v and 'reference_answer' not in v
        assert create()[1]['id']==wid
        assert create(assignment_id=c.assignment())[0]==409
        for who in ['peer','other']:assert req(path,who=who)==req('/api/walkthroughs/9223372036854775807',who=who) and req(path,who=who)[0]==404
        assert req(path,who='teacher')[0]==403
        base={'content':'先观察已知条件','action':'discuss','expected_version':1,'client_key':key+'-first'}
        assert req(turn,{**base,'action':'advance'})[0]==409
        assert req(turn,{**base,'action':'invalid'})[0]==400
        payloads=[]
        for phase in ['read','method','derive','check']:
            v=req(path)[1];payload={**base,'expected_version':v['version'],'client_key':key+'-'+phase}
            code,stream=req(turn,payload);assert code==200 and 'event: done' in stream,(code,stream);payloads.append(payload)
            assert req(path)[1]['stage']==phase
            v=req(path)[1];action='finish' if phase=='check' else 'advance';p={**base,'action':action,'expected_version':v['version'],'client_key':key+'-next-'+phase}
            assert 'event: done' in req(turn,p)[1]
        done=req(path)[1];assert done['stage']=='completed' and len(done['turns'])==8 and done['summary'] and done['improvements']
        calls=gate()['calls'];assert 'event: done' in req(turn,payloads[0])[1] and gate()['calls']==calls
        assert req(turn,{**payloads[0],'content':'different'})[0]==409
        assert 'score' not in json.dumps(done)
        code,v=create(client_key=key+'-failure');ids.append(v['id']);wid=v['id'];path='/api/walkthroughs/'+str(wid);turn=path+'/turns';base={**base,'client_key':key+'-retry'}
        for settings in [{'walk_raw':'bad json'},{'walk_response':{'response':'最终答案是2','summary':'结果','improvements':['检查']}},{'walk_response':{'response':'检查[999]','summary':'结果','improvements':['检查']}}]:
            c.control(**settings);code,stream=req(turn,base);assert code==200 and 'event: error' in stream;v=req(path)[1];assert v['stage']=='read' and v['version']==1 and not v['active'];assert v['turns'][0]['status']=='failed'
        c.control(statuses=[503,200]);assert 'event: done' in req(turn,base)[1] and gate()['calls']==2
        c.control(hold_walk=True);base={**base,'expected_version':2,'client_key':key+'-hold'}
        with ThreadPoolExecutor(max_workers=1) as pool:
            pending=pool.submit(req,turn,base);started();assert req(turn,{**base,'client_key':key+'-other'})[0]==409
            c.sql('UPDATE walkthroughs SET lease_until=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE id='+str(wid));v=req(path)[1];assert not v['active'] and v['turns'][-1]['status']=='failed';release();assert 'event: error' in pending.result()[1]
        c.control(hold_walk=True);base={**base,'client_key':key+'-cancel'}
        connection=http.client.HTTPConnection('127.0.0.1',18081,timeout=5);connection.request('POST',turn,json.dumps(base),{'Content-Type':'application/json','Authorization':'Bearer '+c.token('student')});response=connection.getresponse();assert response.status==200;started();response.close();connection.close()
        for _ in range(50):
            if not req(path)[1]['active']:break
            time.sleep(.05)
        else:raise AssertionError('cancel did not release')
        assert req(path)[1]['turns'][-1]['status']=='cancelled';assert gate()['cancelled']>=1
        c.control();assert 'event: done' in req(turn,base)[1]
        before=c.sql('CHECKSUM TABLE materials,grade_assignments,submissions,grade_results,grade_reviews,walkthroughs,walkthrough_turns')
        for _ in range(2):
            c.compose('restart','api')
            for _ in range(50):
                try:
                    if req(path)[0]==200:break
                except Exception:pass
                time.sleep(.1)
            assert c.sql('CHECKSUM TABLE materials,grade_assignments,submissions,grade_results,grade_reviews,walkthroughs,walkthrough_turns')==before
        assert c.sql("SELECT COUNT(*) FROM curriculum_migrations WHERE version='003_walkthroughs.sql'")[0][0]=='1'
        timings=[c.request('/api/walkthroughs',who='student')[2]*1000 for _ in range(100)];p95=sorted(timings)[94];assert p95<=500
        return {'stages':done['stage'],'successful_turns':8,'private':'same-class and foreign404; teacher403','replay':'no extra call','cancel':'model cancelled; same key retry','expired_lease':'failed; old writer rejected','bad_output':'no stage advance','migration_restarts':2,'list_p95_ms':round(p95,2)}
    finally:
        c.control()
        if ids:c.sql('DELETE FROM walkthroughs WHERE id IN ('+','.join(map(str,ids))+')')
