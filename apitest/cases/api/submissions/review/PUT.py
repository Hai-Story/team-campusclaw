from concurrent.futures import ThreadPoolExecutor

def run(c):
    # A7/A8/A10: real HTTP, class-scoped SQL state and append-only review versions.
    own=[]
    def judge(sid,correct,score,version=0,who='teacher'):
        return c.request(f'/api/submissions/{sid}/review',{'correct':correct,'score':score,'feedback':'终评验证','expected_version':version},who=who,method='PUT')
    def mistakes(subject='math'):
        code,data,_,_=c.request('/api/mistakes?subject='+subject)
        assert code==200
        return {x['assignment_id'] for x in data['items']}
    for ai,final in ((True,True),(True,False),(False,True),(False,False)):
        c.control(delay_ms=10,statuses=[],correct=ai)
        aid=c.assignment();receipt=c.submit(aid)[1];task=c.wait(receipt['task_id'])
        assert task['status']=='succeeded' and task['result']['correct'] is ai and 'score' not in task['result']
        assert c.sql(f"SELECT score IS NULL FROM grade_results WHERE submission_id={receipt['submission_id']};")[0][0]=='1'
        assert (aid in mistakes()) is (not ai)
        code,review,_,_=judge(receipt['submission_id'],final,8 if final else 3)
        assert code==200 and review['correct'] is final and review['version']==1
        latest=c.request(f'/api/assignments/{aid}/my-submission')[1]
        assert latest['id']==receipt['submission_id'] and latest['task']['final_review']['correct'] is final
        assert latest['task']['result']['correct'] is ai
        assert (aid in mistakes()) is (not final)
        listing=c.request(f'/api/assignments/{aid}/submissions',who='teacher')
        assert listing[0]==200 and listing[1]['items'][0]['answer']=='2'
        assert c.request(f'/api/assignments/{aid}/submissions')[0]==403
        assert c.request(f'/api/assignments/{aid}/submissions',who='other')[0]==403
        assert c.request('/api/tasks/'+receipt['task_id'],who='peer')[0]==404
        own.append((aid,receipt))

    aid,receipt=own[-1];sid=receipt['submission_id']
    assert judge(sid,True,10,1)[0]==200 and aid not in mistakes()
    assert judge(sid,False,0,2)[0]==200 and aid in mistakes()
    before=c.sql(f'SELECT version FROM grade_reviews WHERE submission_id={sid}; SELECT COUNT(*) FROM grade_review_history WHERE submission_id={sid};')
    for payload in ({'score':1,'expected_version':3},{'correct':False,'expected_version':3},{'score':1,'correct':False},
                    {'score':-1,'correct':False,'expected_version':3},{'score':11,'correct':True,'expected_version':3},
                    {'score':0,'correct':'false','expected_version':3},{'score':True,'correct':False,'expected_version':3},
                    {'score':0,'correct':False,'expected_version':3,'class_id':1}):
        assert c.request(f'/api/submissions/{sid}/review',payload,who='teacher',method='PUT')[0]==400
    assert judge(sid,False,0,3,who='student')[0]==403
    assert judge(sid,False,0,3,who='other')[0]==403
    assert judge(sid,False,0,2)[0]==409
    assert before==c.sql(f'SELECT version FROM grade_reviews WHERE submission_id={sid}; SELECT COUNT(*) FROM grade_review_history WHERE submission_id={sid};')
    with ThreadPoolExecutor(max_workers=2) as pool:
        codes=list(pool.map(lambda correct:judge(sid,correct,5,3)[0],[True,False]))
    assert sorted(codes)==[200,409]
    assert c.sql(f'SELECT version FROM grade_reviews WHERE submission_id={sid}; SELECT COUNT(*) FROM grade_review_history WHERE submission_id={sid};')==[['4'],['4']]
    # Inject a history-write failure: the current head update must roll back too.
    c.sql('ALTER TABLE grade_review_history ADD COLUMN apitest_required INT NOT NULL;')
    try:assert judge(sid,True,10,4)[0]==503
    finally:c.sql('ALTER TABLE grade_review_history DROP COLUMN apitest_required;')
    assert c.sql(f'SELECT version FROM grade_reviews WHERE submission_id={sid}; SELECT COUNT(*) FROM grade_review_history WHERE submission_id={sid};')==[['4'],['4']]

    # A real foreign-class row in the isolated fixture; no production data involved.
    foreign=int(c.sql("INSERT INTO grade_assignments(class_id,created_by,subject,title,question,expected_answer,max_score) SELECT b.class_id,a.id,'science','foreign-fixture','q','2',10 FROM users b JOIN users a ON a.username='teacher_a' WHERE b.username='student_b1'; SELECT LAST_INSERT_ID();")[0][0]);c.assignments.append(foreign)
    c.control(delay_ms=10,statuses=[],correct=False)
    code,foreign_receipt,_,_=c.request(f'/api/assignments/{foreign}/submissions',{'answer':'2'},who='other',headers={'Idempotency-Key':c.run_id+'-foreign'})
    assert code==202
    assert judge(foreign_receipt['submission_id'],False,0)[0]==404
    assert c.request(f'/api/assignments/{foreign}/submissions',who='teacher')[0]==404
    assert c.request(f'/api/assignments/{foreign}/my-submission')[0]==404
    assert foreign not in mistakes('science')

    c.control(delay_ms=500,statuses=[],correct=False)
    late=c.assignment(subject='science');r=c.submit(late)[1]
    assert judge(r['submission_id'],True,7)[0]==200
    t=c.wait(r['task_id']);assert t['result']['correct'] is False and t['final_review']['correct'] is True and t['final_review']['score']==7
    c.compose('exec','-T','worker','/app/grade-worker','-publish',r['task_id'])
    for _ in range(10):assert c.task(r['task_id'])['final_review']['version']==1
    assert late not in mistakes('science')
    c.control(delay_ms=10,statuses=[],correct=False)
    science=c.assignment(subject='science');r=c.submit(science)[1];c.wait(r['task_id'])
    assert science in mistakes('science') and science not in mistakes('math')
    c.control(delay_ms=10,statuses=[],correct=True)
    r=c.submit(science,answer='3')[1];c.wait(r['task_id'])
    assert science not in mistakes('science'),'latest correct attempt must replace the old mistake'
    c.control(delay_ms=10,statuses=[401])
    failed=c.assignment();r=c.submit(failed)[1];assert c.wait(r['task_id'])['status']=='failed'
    assert judge(r['submission_id'],False,0)[0]==200
    assert failed in mistakes()
    c.control(delay_ms=20,statuses=[])
    return {'judgment_combinations':4,'concurrent_reviews':sorted(codes),'history_version':4,'history_rollback':'PASS','late_AI_override':'blocked','subject_isolation':'PASS','latest_attempt':'PASS','authorization':'403/404','AI_numeric_score':None}
