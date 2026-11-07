import json,uuid

def run(c):
    # A12: a transport retry is one task; a new deliberate attempt invokes the model again.
    c.control(correct=False)
    aid=c.assignment();key=uuid.uuid4().hex
    first=c.submit(aid,key=key)[1];c.wait(first['task_id'])
    assert c.submit(aid,key=key)[1]['task_id']==first['task_id']
    second=c.submit(aid)[1];assert second['task_id']!=first['task_id'];c.wait(second['task_id'])
    sid=second['submission_id']
    assert c.request(f'/api/submissions/{sid}/review',{'score':5,'correct':True,'expected_version':0},who='teacher',method='PUT')[0]==200
    c.sql(f'UPDATE grade_results SET ai_correct=NULL,score=5 WHERE submission_id={sid};')
    old=c.sql(f'SELECT id,feedback,score,created_at FROM grade_results WHERE submission_id={sid};')
    observed=json.loads(c.compose('exec','-T','worker','/app/grade-worker','-refresh-legacy').splitlines()[-1])
    assert observed['updated']==1
    task=c.task(second['task_id']);assert task['result']['correct'] is False and task['final_review']['correct'] is True
    assert c.sql(f'SELECT id,feedback,score,created_at FROM grade_results WHERE id={old[0][0]};')==old
    assert c.sql(f'SELECT COUNT(*),SUM(score IS NULL) FROM grade_results WHERE submission_id={sid};')[0]==['2','1']
    assert json.loads(c.compose('exec','-T','worker','/app/grade-worker','-refresh-legacy').splitlines()[-1])['updated']==0
    third=c.submit(aid)[1];c.wait(third['task_id']);sid3=third['submission_id']
    c.sql(f'UPDATE grade_results SET ai_correct=NULL WHERE submission_id={sid3};')
    before=c.sql(f'SELECT idempotency_key FROM grade_tasks WHERE submission_id={sid3};')
    c.control(statuses=[401])
    try:
        try:c.compose('exec','-T','worker','/app/grade-worker','-refresh-legacy');raise AssertionError('failed model was accepted')
        except RuntimeError:pass
        assert c.sql(f'SELECT idempotency_key FROM grade_tasks WHERE submission_id={sid3};')==before
        assert c.sql(f'SELECT COUNT(*) FROM grade_results WHERE submission_id={sid3};')[0][0]=='1'
    finally:c.control()
    return {'same_key':'same task','new_attempt':'new task','legacy_versions':2,'teacher_review':'preserved','repeat_updated':0,'failed_refresh':'no write'}
