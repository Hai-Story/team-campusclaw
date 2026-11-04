import json,time

def run(c):
    # A2-S4/A3/A4/A5: persistent delivery, crash recovery, terminal dedup and failure classes.
    c.control(delay_ms=20)
    aid=c.assignment()
    c.compose('stop','worker')
    try:
        code,first,_,_=c.submit(aid);assert code==202
        task=first['task_id'];time.sleep(.5)
        assert c.task(task)['status']=='pending'
        for _ in range(10):assert c.task(task)['attempts']==0
    finally:c.compose('start','worker')
    succeeded=c.wait(task);assert succeeded['status']=='succeeded' and succeeded['result']['score']==10
    c.compose('exec','-T','worker','/app/grade-worker','-publish',task)
    time.sleep(.5);assert c.task(task)['result']==succeeded['result']
    assert c.sql(f"SELECT COUNT(*) FROM grade_results WHERE submission_id={first['submission_id']};")[0][0]=='1'
    # A3-S3: two consumers, expired old lease and distinct feedback expose stale writes.
    c.control(delay_ms=2000)
    c.compose('up','-d','--scale','worker=2','worker')
    try:
        fenced=c.submit(aid)[1];c.wait(fenced['task_id'],statuses=('running',))
        assert c.request('/api/tasks/'+fenced['task_id'])[2]<1
        c.sql(f"UPDATE grade_tasks SET lease_until=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) WHERE task_id='{fenced['task_id']}';")
        c.compose('exec','-T','worker','/app/grade-worker','-publish',fenced['task_id'])
        deadline=time.monotonic()+10
        while c.task(fenced['task_id'])['attempts']<2 and time.monotonic()<deadline:time.sleep(.1)
        assert c.task(fenced['task_id'])['attempts']==2
        extra=c.submit(aid);assert extra[0]==202 and extra[2]<1
        fenced_result=c.wait(fenced['task_id']);assert fenced_result['status']=='succeeded' and fenced_result['result']['feedback'].startswith('第2次')
        assert c.sql(f"SELECT COUNT(*) FROM grade_results WHERE submission_id={fenced['submission_id']};")[0][0]=='1'
        c.wait(extra[1]['task_id'])
    finally:c.compose('up','-d','--scale','worker=1','worker')
    # A claim interrupted before result commit must be reclaimed after its lease.
    c.control(delay_ms=2000)
    crashing=c.submit(aid)[1];c.wait(crashing['task_id'],statuses=('running',))
    c.compose('kill','-s','SIGKILL','worker');c.compose('start','worker')
    recovered=c.wait(crashing['task_id']);assert recovered['status']=='succeeded' and recovered['attempts']>=2
    c.compose('exec','-T','worker','/app/grade-worker','-publish',crashing['task_id'])
    time.sleep(.3);assert c.task(crashing['task_id'])['result']==recovered['result']
    # A reachable broker is not required to durably accept a new submission.
    c.control(delay_ms=20)
    c.compose('stop','rabbitmq')
    try:
        unavailable=c.submit(aid);assert unavailable[0]==202
        assert c.task(unavailable[1]['task_id'])['status']=='pending'
    finally:c.compose('start','rabbitmq')
    assert c.wait(unavailable[1]['task_id'])['status']=='succeeded'
    c.control(delay_ms=20,statuses=[503,503,200])
    retry=c.submit(aid)[1];retried=c.wait(retry['task_id']);assert retried['status']=='succeeded' and retried['attempts']==3
    c.control(delay_ms=20,statuses=[503])
    failure=c.submit(aid)[1];failed=c.wait(failure['task_id']);assert failed['status']=='failed' and failed['attempts']==3 and 'result' not in failed
    c.control(delay_ms=20,statuses=[401])
    permanent=c.submit(aid)[1];rejected=c.wait(permanent['task_id']);assert rejected['status']=='failed' and rejected['attempts']==1
    assert c.sql(f"SELECT COUNT(*) FROM grade_results WHERE submission_id IN ({failure['submission_id']},{permanent['submission_id']});")[0][0]=='0'
    deadline=time.monotonic()+10
    while time.monotonic()<deadline:
        dead=c.sql(f"SELECT COUNT(*) FROM grade_outbox WHERE task_id IN ('{failure['task_id']}','{permanent['task_id']}') AND kind='dead' AND state='published';")[0][0]
        if dead=='2':break
        time.sleep(.2)
    assert dead=='2'
    c.compose('exec','-T','worker','/app/grade-worker','-migrate')
    c.compose('exec','-T','worker','/app/grade-worker','-migrate')
    c.control(delay_ms=20)
    return {'stopped_consumer':'pending then succeeded','fenced_attempts':fenced_result['attempts'],'stale_writer':'discarded','crash_attempts':recovered['attempts'],'duplicate_results':1,'broker_recovery':'PASS','retry_attempts':retried['attempts'],'permanent_attempts':rejected['attempts'],'dead_events_published':int(dead),'migration_repeats':2}
