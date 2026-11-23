"""D2: deterministic teacher-final statistics, scopes, latest heads and calendar dates."""
from datetime import datetime,timedelta,timezone
import statistics,time

def run(c):
    points=[]
    def req(path='/api/learning-diagnosis',who='teacher',**kwargs):return c.request(path,who=who,**kwargs)[:2]
    def report(who='teacher',suffix=''):
        code,data=req('/api/learning-diagnosis?subject=math&days=30'+suffix,who);assert code==200,(code,data);return data
    def pstat(r,pid):return next(x for x in r['points'] if x['id']==pid)
    def review(sid,score,correct,version=0,tags=None):
        payload={'score':score,'correct':correct,'expected_version':version}
        if tags is not None:payload['error_tags']=tags
        r=c.request(f'/api/submissions/{sid}/review',payload,who='teacher',method='PUT');assert r[0]==200,r[:2];return r[1]
    try:
        for who in ['teacher','student','peer','other']:c.token(who)
        student=int(c.sql("SELECT id FROM users WHERE username='student_a1'")[0][0]);foreign=int(c.sql("SELECT id FROM users WHERE username='student_b1'")[0][0])
        assert req(who=None)[0]==401
        for query in ['subject=all','days=0','days=181','days=1.5','page=0','page_size=0','role=teacher','class_id=1','days=1&days=2','student_id=-1']:
            assert req('/api/learning-diagnosis?'+query)[0]==400,query
        assert req('/api/learning-diagnosis?student_id='+str(student),who='student')[0]==400
        assert req('/api/learning-diagnosis?student_id='+str(foreign))==req('/api/learning-diagnosis?student_id=999999999')
        assert req('/api/learning-diagnosis?student_id='+str(foreign))[0]==404
        base=report();c.control(correct=False)
        a=c.assignment(title='apitest-'+c.run_id+'-diagnosis-a')
        code,b,_,_=c.request('/api/assignments',{'subject':'math','title':'apitest-'+c.run_id+'-diagnosis-b','question':'合成问题','expected_answer':'参考','max_score':20},who='teacher');assert code==201;b=b['id'];c.assignments.append(b)
        for i,aids in enumerate([[a,b],[a]]):
            code,p,_,_=c.request('/api/knowledge-points',{'subject':'math','name':'诊断合成-'+c.run_id+'-'+str(i),'chapter':'合成章节','description':'只用于隔离验证','material_ids':[],'assignment_ids':aids},who='teacher');assert code==201;points.append(p['id'])
        s1=c.submit(a,answer='暂定作答')[1];c.wait(s1['task_id']);assert pstat(report(),points[0])['samples']==0
        s2=c.submit(b,answer='合成作答')[1]
        peer=c.request(f'/api/assignments/{a}/submissions',{'answer':'同班合成作答'},who='peer',headers={'Idempotency-Key':c.run_id+'-peer'})[1]
        review(s1['submission_id'],4,False,tags=['concept']);review(s2['submission_id'],20,True);review(peer['submission_id'],10,True)
        r=report();ps=pstat(r,points[0]);assert (ps['samples'],ps['score'],ps['max_score'],ps['correct'],ps['score_percent'],ps['correct_percent'])==(3,34,40,2,85,66.67),ps
        ps2=pstat(r,points[1]);assert ps2['samples']==2 and ps2['score_percent']==70 and ps2['correct_percent']==50
        assert r['samples']==base['samples']+3 and r['error_counts']['concept']==base['error_counts']['concept']+1
        mine=pstat(report('student'),points[0]);assert mine['samples']==2 and mine['score_percent']==80
        assert pstat(report('peer'),points[0])['samples']==1
        assert points[0] not in {p['id'] for p in report('other')['points']}
        own_report=report('student');assert [x['id'] for x in own_report['students']]==[student]
        selected=report(suffix='&student_id='+str(student));assert pstat(selected,points[0])==mine
        unknown=req('/api/learning-diagnosis/samples/999999999',who='peer');assert req('/api/learning-diagnosis/samples/'+str(s1['submission_id']),who='peer')==unknown and unknown[0]==404
        assert req('/api/learning-diagnosis/samples/'+str(s1['submission_id']),who='other')[0]==404
        code,detail=req('/api/learning-diagnosis/samples/'+str(s1['submission_id']),who='student');assert code==200 and detail['answer']=='暂定作答' and 'expected_answer' not in detail and detail['review_version']==1 and set(detail['knowledge_point_ids'])==set(points)
        review(s1['submission_id'],10,True,1);corrected=report();assert corrected['fingerprint']!=r['fingerprint'] and pstat(corrected,points[0])['score_percent']==100
        assert corrected['error_counts']['concept']==base['error_counts']['concept']
        newer=c.submit(a,answer='新作答')[1];assert pstat(report(),points[0])['samples']==2
        assert req('/api/learning-diagnosis/samples/'+str(s1['submission_id']))[0]==404
        review(newer['submission_id'],0,False,tags=[]);r=report();assert pstat(r,points[0])['samples']==3 and r['error_counts']['unclassified']==base['error_counts']['unclassified']+1
        # Exact Chinese calendar boundary is based on submission time, not review time.
        china=timezone(timedelta(hours=8));now=datetime.now(timezone.utc);today=now.astimezone(china).replace(hour=0,minute=0,second=0,microsecond=0).astimezone(timezone.utc)
        inside=today.strftime('%Y-%m-%d %H:%M:%S.%f');outside=(today-timedelta(microseconds=1)).strftime('%Y-%m-%d %H:%M:%S.%f')
        c.sql(f"UPDATE submissions SET created_at='{inside}' WHERE id={s2['submission_id']}; UPDATE submissions SET created_at='{outside}' WHERE id={peer['submission_id']}")
        day=req('/api/learning-diagnosis?subject=math&days=1')[1];assert pstat(day,points[0])['samples']==2 and len(day['trend'])==1
        assert pstat(report(),points[0])['samples']==3
        paged=report(suffix='&page_size=1');next_page=report(suffix='&page_size=1&page=2');assert paged['fingerprint']==next_page['fingerprint'] and paged['evidence'][0]['submission_id']!=next_page['evidence'][0]['submission_id']
        assert report(suffix='&page_size=101')['page_size']==100
        c.control()
        for i in range(100):
            aid=c.assignment(title='apitest-'+c.run_id+'-perf-'+str(i));s=c.submit(aid)[1];review(s['submission_id'],5,True)
        for _ in range(5):report()
        times=[]
        for _ in range(100):
            start=time.perf_counter();report();times.append((time.perf_counter()-start)*1000)
        p95=sorted(times)[94];assert p95<=500,p95
        snapshot=report()['fingerprint'];c.compose('stop','db')
        try:assert req()[0]==503
        finally:
            c.compose('start','db')
            for _ in range(120):
                try:
                    if req()[0]==200:break
                except Exception:pass
                time.sleep(.25)
            else:raise AssertionError('Database did not recover')
        assert report()['fingerprint']==snapshot
        return {'counting':'weighted score and distinct samples; overlapping point links','AI':'excluded','history':'current head only','latest':'new ungraded attempt excludes old grade','authorization':'own/current-class only; same404','calendar':'China boundary inclusive/exclusive','list_p95_ms':round(p95,2),'database':'503/recovery and same fingerprint'}
    finally:
        for pid in points:c.sql('DELETE FROM knowledge_points WHERE id='+str(pid))
