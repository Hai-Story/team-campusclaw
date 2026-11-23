"""S1/S2: persisted seven-column metadata, scoped queries, denial, failure and migration."""
import datetime,json,time,urllib.parse

def run(c):
    c.compose('start','worker');c.control()
    for who in ['teacher','student','other']:c.token(who)
    uid=int(c.sql("SELECT id FROM users WHERE username='student_a1'")[0][0]);cls=int(c.sql("SELECT class_id FROM users WHERE id="+str(uid))[0][0])
    def read(q=''):return c.request('/api/audit-events'+q,who='teacher')
    assert c.request('/api/audit-events',who=None)[0]==401
    code,_,_,h=c.request('/api/audit-events',who='student');assert code==403
    rid=next(v for k,v in h.items() if k.lower()=='x-request-id')
    for _ in range(100):
        code,data,_,_=read('?request_id='+rid)
        if data['items'] and data['items'][0]['result']=='denied':break
        time.sleep(.02)
    row=data['items'][0];assert row['actor_id']==uid and row['http_status']==403 and row['result']=='denied'
    assert set(['actor_id','time','action','object','result','skill_id','model','duration_ms'])<=set(row)
    assert row['skill_id'] is None and row['model'] is None
    assert c.request('/api/audit-events/'+str(row['id']),who='teacher')[1]['id']==row['id']
    other_id=int(c.sql("SELECT id FROM operation_audit_events WHERE class_id<>"+str(cls)+" AND class_id>0 ORDER BY id DESC LIMIT 1")[0][0])
    assert c.request('/api/audit-events/'+str(other_id),who='teacher')[:2]==c.request('/api/audit-events/999999999',who='teacher')[:2]
    for q in ['?page=0','?page=1.5','?page_size=-1','?actor_id=x','?skill_id=%0Axxx','?sort=body','?result=no','?page=1&page=2','?from=2000-01-01T00:00:00Z']:
        assert read(q)[0]==400,q
    assert read('?page_size=101')[1]['page_size']==100
    now=datetime.datetime.now(datetime.timezone.utc);q=urllib.parse.urlencode({'actor_id':uid,'from':(now-datetime.timedelta(minutes=10)).isoformat(),'to':(now+datetime.timedelta(seconds=1)).isoformat()})
    code,v,_,_=read('?'+q);assert code==200 and v['items'] and all(x['actor_id']==uid and x['class_id']==cls for x in v['items'])
    a=c.assignment();code,receipt,_,_=c.submit(a);assert code==202;c.wait(receipt['task_id'])
    for _ in range(100):
        _,v,_,_=read('?skill_id=grading-verdict-v3')
        matching=[x for x in v['items'] if x['request_id']==receipt['task_id']]
        if matching and matching[0]['result']=='succeeded':break
        time.sleep(.03)
    event=matching[0];assert event['skill_id']=='grading-verdict-v3' and event['model']['usage']['reported'] is True and event['model']['usage']['total_tokens']>0,event
    assert int(c.sql("SELECT COUNT(*) FROM operation_audit_events WHERE event_key='grade:"+receipt['task_id']+":1'")[0][0])==1
    first=read('?page_size=2')[1];second=read('?page_size=2&page=2&before_id='+str(first['snapshot_id']))[1]
    assert not set(x['id'] for x in first['items'])&set(x['id'] for x in second['items'])
    # Repeat additive migration via the installed worker; preserve one exact JSON row.
    before=c.sql('SELECT payload FROM operation_audit_events WHERE id='+str(row['id']))
    for _ in range(2):c.compose('exec','-T','worker','/app/grade-worker','-migrate')
    assert c.sql('SELECT payload FROM operation_audit_events WHERE id='+str(row['id']))==before
    # Fail closed before handler; no assignment can be inserted without a reserved audit row.
    count=c.sql('SELECT COUNT(*) FROM grade_assignments')
    try:
        c.sql('RENAME TABLE operation_audit_events TO operation_audit_events_unavailable')
        assert c.request('/api/assignments',{'subject':'math','title':'must-not-insert','question':'1+1','expected_answer':'2','max_score':10},who='teacher')[0]==503
        assert c.sql('SELECT COUNT(*) FROM grade_assignments')==count
        assert c.request('/live',who=None)[0]==200
    finally:c.sql('RENAME TABLE operation_audit_events_unavailable TO operation_audit_events')
    # Identity binding failure must also prevent the handler; finalization failure
    # preserves a truthful started record instead of silently claiming success.
    try:
        cutoff=c.sql('SELECT UTC_TIMESTAMP(6)')[0][0]
        c.sql("ALTER TABLE operation_audit_events ADD CONSTRAINT audit_bind_failure CHECK(class_id=0 OR created_at<'"+cutoff+"')")
        assert c.request('/api/assignments',{'subject':'math','title':'must-not-insert','question':'1+1','expected_answer':'2','max_score':10},who='teacher')[0]==503
        assert c.sql('SELECT COUNT(*) FROM grade_assignments')==count
    finally:c.sql('ALTER TABLE operation_audit_events DROP CHECK audit_bind_failure')
    try:
        cutoff=c.sql('SELECT UTC_TIMESTAMP(6)')[0][0]
        c.sql("ALTER TABLE operation_audit_events ADD CONSTRAINT audit_finish_failure CHECK(result='started' OR created_at<'"+cutoff+"')")
        code,_,_,h=c.request('/api/materials',who='teacher');assert code==200
        rid=next(v for k,v in h.items() if k.lower()=='x-request-id');time.sleep(.05)
        stored=c.sql("SELECT result,actor_id FROM operation_audit_events WHERE event_key='http:"+rid+"'")[0];assert stored[0]=='started' and int(stored[1])>0,stored
    finally:c.sql('ALTER TABLE operation_audit_events DROP CHECK audit_finish_failure')
    c.compose('stop','worker')
    aid=c.assignment();code,receipt,_,_=c.submit(aid);assert code==202
    calls=c.fixture('/stats')['calls'];constraint_added=False
    try:
        c.sql("ALTER TABLE operation_audit_events ADD CONSTRAINT audit_reserve_failure CHECK(event_key NOT LIKE 'grade:"+receipt['task_id']+":%')");constraint_added=True
        c.compose('start','worker');time.sleep(3)
        state=c.sql("SELECT status,attempts FROM grade_tasks WHERE task_id='"+receipt['task_id']+"'")[0]
        assert state[0]=='pending' and int(state[1])==0,state
        assert c.fixture('/stats')['calls']==calls
    finally:
        try:
            if constraint_added:c.sql('ALTER TABLE operation_audit_events DROP CHECK audit_reserve_failure')
        finally:c.compose('start','worker')
    assert c.wait(receipt['task_id'])['status']=='succeeded'
    times=[]
    for i in range(105):
        code,_,elapsed,_=read('?actor_id='+str(uid));assert code==200
        if i>=5:times.append(elapsed*1000)
    p95=sorted(times)[94];assert p95<=500,('p95',p95)
    raw=json.dumps(read('?actor_id='+str(uid))[1]);assert c.token('student') not in raw
    return {'seven_columns':'persisted','denial':{'actor_id':uid,'http_status':403,'audit_id':row['id']},'class_scope':'401/403/same404','queries':'actor+time, skill+time; stable snapshot paging','migration':'repeat2; JSON preserved; audit reserve outage does not consume attempt budget','storage_failure':'503 before business; live200','list_p95_ms':round(p95,2),'warning':p95>300}
