import math,time

def run(c):
    # A2-S1: nearest-rank P95 over exactly twenty accepted requests at each delay.
    observations=[]
    for delay in (2000,4000):
        c.control(delay_ms=delay)
        aid=c.assignment()
        warm=c.submit(aid)[1];assert c.wait(warm['task_id'])['status']=='succeeded'
        execution=float(c.sql(f"SELECT TIMESTAMPDIFF(MICROSECOND,started_at,finished_at)/1000 FROM grade_tasks WHERE task_id='{warm['task_id']}';")[0][0])
        samples=[]
        for _ in range(20):
            code,receipt,elapsed,_=c.submit(aid)
            assert code==202 and 'result' not in receipt and 'score' not in receipt and 'feedback' not in receipt
            samples.append(round(elapsed*1000,3))
        p95=sorted(samples)[math.ceil(.95*len(samples))-1]
        assert p95<1000 and p95<execution/2,(p95,execution)
        queues=c.queues()
        observations.append({'gateway_delay_ms':delay,'background_execution_ms':execution,'samples_ms':samples,'p95_ms':p95,'queues':queues})
        # Only remove this case's synthetic workload; dangling deliveries are safely acknowledged.
        c.compose('stop','worker');c.discard(aid);c.compose('start','worker')
        deadline=time.monotonic()+15
        while c.queues()['grade.jobs']['ready_messages'] and time.monotonic()<deadline:time.sleep(.2)
    # A2-S3/A3-S1: atomic durable intent while both relay and worker are stopped.
    c.control(delay_ms=20);aid=c.assignment()
    c.compose('stop','relay','worker')
    try:
        accepted=c.submit(aid);assert accepted[0]==202
        task=accepted[1]['task_id'];sid=accepted[1]['submission_id']
        before=c.sql(f"SELECT state,publish_attempts FROM grade_outbox WHERE task_id='{task}' AND kind='work';")[0]
        assert before==['pending','0'] and c.task(task)['status']=='pending'
    finally:c.compose('start','relay','worker')
    result=c.wait(task);assert result['status']=='succeeded' and result['result']['correct'] is True and 'score' not in result['result']
    after=c.sql(f"SELECT state,publish_attempts FROM grade_outbox WHERE task_id='{task}' AND kind='work';")[0]
    assert after==['published','1']
    counts=c.sql(f'SELECT (SELECT COUNT(*) FROM submissions WHERE id={sid}),(SELECT COUNT(*) FROM grade_tasks WHERE submission_id={sid}),(SELECT COUNT(*) FROM grade_results WHERE submission_id={sid});')[0]
    assert counts==['1','1','1']
    return {'response_measurements':observations,'stopped_relay_outbox':before,'resumed_outbox':after,'transaction_rows':counts,'ai_correct':result['result']['correct']}
