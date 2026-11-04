from concurrent.futures import ThreadPoolExecutor

def check(c):
    # A1-S1/S2, A2-S2/S3, A4-S1/S2: actual HTTP and relational state.
    aid=c.assignment()
    assert c.request(f'/api/assignments/{aid}')[0]==200
    assert 'expected_answer' not in c.request(f'/api/assignments/{aid}')[1]
    assert c.request(f'/api/assignments/{aid}',who='other')[0]==404
    assert c.request('/api/assignments',who=None)[0]==401
    assert c.request('/api/assignments',{'title':'x'},who='student')[0]==403
    assert c.request('/api/assignments',{'title':' ','question':'1','expected_answer':'2','max_score':101},who='teacher')[0]==400
    for query in ('page=0','page_size=-1','page_size=1.5','sort=secret'):
        assert c.request('/api/assignments?'+query)[0]==400
    assert c.request('/api/assignments?page_size=101')[1]['page_size']==100
    key=c.run_id+'-repeat'
    first=c.submit(aid,key);assert first[0]==202 and 'score' not in first[1]
    for _ in range(2):assert c.submit(aid,key)[1]['task_id']==first[1]['task_id']
    c.token('student')
    with ThreadPoolExecutor(max_workers=2) as pool:
        duplicates=list(pool.map(lambda _:c.submit(aid,key),range(2)))
    assert all(result[0]==202 and result[1]['task_id']==first[1]['task_id'] for result in duplicates)
    assert c.submit(aid,key,'3')[0]==409
    sid=first[1]['submission_id'];task=first[1]['task_id']
    counts=c.sql(f"SELECT (SELECT COUNT(*) FROM submissions WHERE id={sid}),(SELECT COUNT(*) FROM grade_tasks WHERE submission_id={sid}),(SELECT COUNT(*) FROM grade_outbox WHERE task_id='{task}');")[0]
    assert counts==['1','1','1'],counts
    before=c.task(task)['attempts']
    for _ in range(10):assert c.task(task)['attempts']==before
    assert c.request('/api/tasks/'+task,who='other')[0]==404
    assert c.request('/api/tasks/'+task,who='peer')[0]==404
    assert c.request('/api/tasks/'+task,who=None)[0]==401
    assert c.request('/api/tasks/invalid')[0]==400
    assert c.request('/api/tasks/'+task,who='other')[1]==c.request('/api/tasks/'+'f'*32,who='other')[1]
    baseline=c.sql(f'SELECT COUNT(*) FROM submissions WHERE assignment_id={aid};')[0][0]
    # An extra NOT NULL column without a default rejects new event inserts under strict SQL mode.
    # It needs no elevated privileges and is removed even when the assertion fails.
    c.sql('ALTER TABLE grade_outbox ADD COLUMN apitest_required INT NOT NULL;')
    try:assert c.submit(aid,c.run_id+'-rollback')[0]==503
    finally:c.sql('ALTER TABLE grade_outbox DROP COLUMN apitest_required;')
    assert c.sql(f'SELECT COUNT(*) FROM submissions WHERE assignment_id={aid};')[0][0]==baseline
    assert c.sql('SELECT COUNT(*) FROM grade_tasks t LEFT JOIN submissions s ON s.id=t.submission_id WHERE s.id IS NULL;')[0][0]=='0'
    assert c.sql('SELECT COUNT(*) FROM grade_outbox o LEFT JOIN grade_tasks t ON t.task_id=o.task_id WHERE t.task_id IS NULL;')[0][0]=='0'
    assert c.submit(aid,c.run_id+'-rollback')[0]==202
    return {'assignment_id':aid,'idempotent_submission':sid,'rows':counts,'rollback':'no orphan rows','authorization':'401/403/404','query_side_effects':0}

def run(c):
    has_worker='worker' in c.compose('config','--services').splitlines()
    if has_worker:c.compose('stop','worker')
    try:return check(c)
    finally:
        if has_worker:c.compose('start','worker')
