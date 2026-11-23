"""N1-N3: transactional publication, student isolation and durable read state."""
import json, time
from concurrent.futures import ThreadPoolExecutor

def run(c):
    point=paper=draft=pub=None;children=[];constraint=None;renamed=False
    prefix='notice-'+c.run_id
    def req(path,body=None,who='student',method=None,**kw):return c.request(path,body,who,method,**kw)[:2]
    def notices(who='student',query=''):return req('/api/homework-notifications'+query,who=who)[1]
    def find(resource,kind='assignment',who='student'):return next(v for v in notices(who,'?page_size=100')['items'] if v['resource_id']==resource and v['kind']==kind)
    def read(n,who='student'):return req('/api/homework-notifications/'+str(n)+'/read',who=who,method='PUT')
    try:
        for who in ['teacher','student','peer','other']:c.token(who)
        assert req('/api/homework-notifications',who=None)[0]==401
        assert req('/api/homework-notifications',who='teacher')[0]==403
        before={who:notices(who)['unread_count'] for who in ['student','peer','other']}
        aid=c.assignment(prefix)
        a=find(aid);b=find(aid,who='peer')
        assert a['id']!=b['id'] and a['read_at'] is None and b['read_at'] is None
        assert set(a)=={'id','kind','resource_id','subject','title','created_at','read_at'}
        assert a['title']==prefix and a['subject']=='math'
        assert notices()['unread_count']==before['student']+1 and notices('peer')['unread_count']==before['peer']+1
        assert notices('other')['unread_count']==before['other']
        missing=read(999999999);assert missing[0]==404
        assert read(b['id'])==missing and read(a['id'],'other')==missing
        assert read(a['id'],'teacher')[0]==403 and read(a['id'],None)[0]==401
        for n in [0,-1,'x']:assert read(n)[0]==400
        with ThreadPoolExecutor(max_workers=4) as pool:assert list(pool.map(lambda _:read(a['id'])[0],range(4)))==[204]*4
        first=find(aid)['read_at'];assert first
        time.sleep(.02);assert read(a['id'])[0]==204 and find(aid)['read_at']==first
        assert notices()['unread_count']==before['student'] and find(aid,who='peer')['read_at'] is None
        assert req('/api/logout',{},method='POST')[0]==200;c.tokens.pop('student');assert find(aid)['read_at']==first
        for q in ['?page=0','?page=-1','?page=x','?page=1.5','?page=1&page=2','?page_size=0','?page_size=','?student_id=1']:
            assert req('/api/homework-notifications'+q)[0]==400,q
        assert notices(query='?page_size=101')['page_size']==100
        assert notices(query='?page=2147483647')['items']==[]
        # Storage failure must roll back the published assignment as well as notices.
        existing=','.join(r[0] for r in c.sql('SELECT DISTINCT assignment_id FROM student_homework_notifications WHERE assignment_id IS NOT NULL')) or '0'
        constraint='notice_reject_'+c.run_id;c.sql(f'ALTER TABLE student_homework_notifications ADD CONSTRAINT {constraint} CHECK(assignment_id IS NULL OR assignment_id IN ({existing}))')
        state=c.sql('SELECT COUNT(*) FROM grade_assignments;SELECT COUNT(*) FROM student_homework_notifications;')
        assert req('/api/assignments',{'subject':'math','title':prefix+'-rollback','question':'q','expected_answer':'a','max_score':1},who='teacher')[0]==503
        assert state==c.sql('SELECT COUNT(*) FROM grade_assignments;SELECT COUNT(*) FROM student_homework_notifications;')
        c.sql(f'ALTER TABLE student_homework_notifications DROP CHECK {constraint}');constraint=None
        # A whole paper produces one notice per student, none for its child assignments.
        mid=req('/api/materials',who='teacher')[1][0]['id']
        code,point_data=req('/api/knowledge-points',{'subject':'math','name':prefix,'chapter':'通知验证','description':'合成知识点','material_ids':[mid],'assignment_ids':[]},who='teacher');assert code==201;point=point_data['id']
        items=[{'type':'short_answer','stem':prefix+' paper '+str(i),'options':[],'answer':['参考'],'explanation':'说明[1]','knowledge_point_ids':[point],'difficulty':'medium','citation_indices':[1]} for i in range(2)]
        c.control(bank_response={'questions':items})
        code,v=req('/api/question-drafts/generate',{'subject':'math','material_ids':[mid],'knowledge_point_ids':[point],'question_types':['short_answer'],'difficulty':'medium','question_count':2,'client_key':prefix},who='teacher',timeout=130)
        assert code==200,(code,v);draft=v['draft']['id'];assert req(f'/api/question-drafts/{draft}/approve',{'expected_version':1},who='teacher')[0]==200
        ids=req('/api/question-drafts/'+str(draft),who='teacher')[1]['bank_ids']
        code,p=req('/api/paper-drafts',{'subject':'math','title':prefix+' paper','items':[{'question_id':i,'question_version':1,'score':10} for i in ids],'client_key':prefix},who='teacher');assert code==201;paper=p['id']
        path=f'/api/paper-drafts/{paper}/publish'
        existing_papers=','.join(r[0] for r in c.sql('SELECT DISTINCT paper_id FROM student_homework_notifications WHERE paper_id IS NOT NULL')) or '0'
        constraint='notice_reject_'+c.run_id;c.sql(f'ALTER TABLE student_homework_notifications ADD CONSTRAINT {constraint} CHECK(paper_id IS NULL OR paper_id IN ({existing_papers}))')
        state=c.sql('SELECT COUNT(*) FROM published_papers;SELECT COUNT(*) FROM grade_assignments;SELECT COUNT(*) FROM student_homework_notifications;')
        assert req(path,{'expected_version':1},who='teacher')[0]==503
        assert state==c.sql('SELECT COUNT(*) FROM published_papers;SELECT COUNT(*) FROM grade_assignments;SELECT COUNT(*) FROM student_homework_notifications;')
        c.sql(f'ALTER TABLE student_homework_notifications DROP CHECK {constraint}');constraint=None
        with ThreadPoolExecutor(max_workers=2) as pool:published=list(pool.map(lambda _:req(path,{'expected_version':1},who='teacher'),range(2)))
        assert [x[0] for x in published]==[201,201] and published[0][1]==published[1][1]
        pub=published[0][1]['id'];children=[q['assignment_id'] for q in published[0][1]['questions']]
        assert c.sql('SELECT COUNT(*) FROM student_homework_notifications WHERE paper_id='+str(pub))==[['2']]
        assert c.sql('SELECT COUNT(*) FROM student_homework_notifications WHERE assignment_id IN ('+','.join(map(str,children))+')')==[['0']]
        pn=find(pub,'paper');assert read(pn['id'])[0]==204
        assert req('/api/published-papers/'+str(pub))[0]==200
        # 20 newest rows, ordered pages, and 100 actual HTTP requests after five warmups.
        for i in range(20):c.assignment(prefix+'-page-'+str(i))
        first=notices();second=notices(query='?page=2')
        assert first['page_size']==20 and len(first['items'])==20
        assert min(v['id'] for v in first['items'])>max(v['id'] for v in second['items'])
        for _ in range(5):notices()
        times=[c.request('/api/homework-notifications')[2]*1000 for _ in range(100)];p95=sorted(times)[94];assert p95<=500
        # Table unavailable: an explicit failure, not a false empty list or successful write.
        c.sql('RENAME TABLE student_homework_notifications TO notice_unavailable_test');renamed=True
        assert req('/api/homework-notifications')[0]==503 and read(a['id'])[0]==503
        c.sql('RENAME TABLE notice_unavailable_test TO student_homework_notifications');renamed=False
        assert find(aid)['read_at'] is not None
        snapshot=c.sql('SELECT * FROM student_homework_notifications ORDER BY id;SELECT id,title,question,expected_answer FROM grade_assignments ORDER BY id;')
        for _ in range(2):
            c.compose('restart','api')
            for attempt in range(100):
                try:
                    if req('/live',who=None)[0]==200:break
                except Exception:pass
                time.sleep(.2)
            else:raise AssertionError('API restart unavailable')
        assert snapshot==c.sql('SELECT * FROM student_homework_notifications ORDER BY id;SELECT id,title,question,expected_answer FROM grade_assignments ORDER BY id;')
        assert c.sql("SELECT COUNT(*) FROM curriculum_migrations WHERE version='010_homework_notifications.sql'")==[['1']]
        c.discard(aid);assert req('/api/homework-notifications/'+str(a['id'])+'/read',method='PUT')==missing
        c.sql('DELETE FROM published_papers WHERE id='+str(pub));pub=None;assert read(pn['id'])==missing
        return {'publication':'single and paper; both class students; one per paper; replay no duplicates; write failure fully rolls back','privacy':'own/current class only; same404; anonymous401; teacher403; no answers or identity fields','read':'four concurrent204, first timestamp stable, peer unread, relogin durable','list':'20/default,100/cap,sorted pages, invalid400, dependency503, cascade cleanup','migration':'restart twice preserves notifications and assignments; version once','list_p95_ms':round(p95,2),'requests':100}
    finally:
        if renamed:c.sql('RENAME TABLE notice_unavailable_test TO student_homework_notifications')
        if constraint:c.sql(f'ALTER TABLE student_homework_notifications DROP CHECK {constraint}')
        if pub:c.sql('DELETE FROM published_papers WHERE id='+str(pub))
        for id in children:c.sql('DELETE FROM grade_assignments WHERE id='+str(id))
        if paper:c.sql('DELETE FROM paper_drafts WHERE id='+str(paper))
        if draft:c.sql('DELETE FROM bank_questions WHERE origin_draft_id='+str(draft)+';DELETE FROM question_drafts WHERE id='+str(draft))
        if point:c.sql('DELETE FROM knowledge_points WHERE id='+str(point))
        c.control()
