"""R1-R3: source reconciliation, deterministic feedback, fresh-session transactions and ownership."""
import json,time,uuid
from datetime import datetime,timedelta,timezone
from concurrent.futures import ThreadPoolExecutor

def run(c):
    points=[];constraints=[];newcards=[]
    def parse_time(value):return datetime.strptime(value,"%Y-%m-%dT%H:%M:%S.%f%z" if "." in value else "%Y-%m-%dT%H:%M:%S%z")
    uid,classid,tid=map(int,c.sql("SELECT s.id,s.class_id,t.id FROM users s JOIN users t ON t.username='teacher_a' WHERE s.username='student_a1'")[0])
    prior=c.sql(f'SELECT HEX(intervals_json),version,created_at,updated_at FROM review_settings WHERE student_id={uid} AND class_id={classid}')
    before={int(r[0]) for r in c.sql(f'SELECT id FROM review_cards WHERE student_id={uid}')}
    def req(path,body=None,**kw):return c.request(path,body,**kw)[:2]
    def sync(who='student'):assert req('/api/review-cards/sync',{},who=who)[0]==200
    def report(subject='math',status='all',**kw):
        code,r=req(f'/api/review-cards?subject={subject}&status={status}&page_size=100',**kw);assert code==200,(code,r);return r
    def cards():return report()['items']
    def card(kind,id):return next(x for x in cards() if x['source_kind']==kind and x['source_id']==id)
    def complete(v,feedback='good',key=None,who='student',**extra):return req('/api/review-cards/'+str(v['id'])+'/complete',{'feedback':feedback,'client_key':key or uuid.uuid4().hex,'expected_version':v['version'],**extra},who=who)
    def point(subject='math',name='concept'):
        code,p=req('/api/knowledge-points',{'subject':subject,'name':'review-'+c.run_id+'-'+name,'chapter':'合成','description':'合成复习知识点','material_ids':[],'assignment_ids':[]},who='teacher');assert code==201,(code,p);points.append(p['id']);assert req(f'/api/knowledge-points/{p["id"]}/favorite',method='PUT')[0]==204;return p['id']
    try:
        for who in ['teacher','student','peer','other']:c.token(who)
        assert req('/api/review-cards',who=None)[0]==401
        for path in ['/api/review-cards','/api/review-settings','/api/review-completions','/api/review-cards/count']:assert req(path,who='teacher')[0]==403
        assert req('/api/review-cards/sync',{},who='teacher')[0]==403
        for q in ['subject=all','class_id=1','student_id=1','page=0','page_size=0','page=1.5','status=other','subject=math&subject=english']:
            assert req('/api/review-cards?'+q)[0]==400,q
        for body in [None,[],{'student_id':uid}]:assert req('/api/review-cards/sync',body,method='POST')[0]==400
        settings=req('/api/review-settings')[1]
        if settings['intervals']!=[1,3,7,14,30]:assert req('/api/review-settings',{'intervals':[1,3,7,14,30],'expected_version':settings['version']},method='PUT')[0]==200
        mid=point();science=point('science','science');sync();a=card('favorite',mid);same=c.sql('CHECKSUM TABLE review_cards');sync();assert card('favorite',mid)==a and c.sql('CHECKSUM TABLE review_cards')==same
        assert any(v['source_id']==science for v in report('science')['items'])
        assert mid not in {v['source_id'] for v in report(who='peer')['items']};assert mid not in {v['source_id'] for v in report(who='other')['items']}
        missing=req('/api/review-cards/999999999/complete',{'feedback':'good','client_key':'private','expected_version':1},who='peer')
        assert missing[0]==404 and complete(a,key='private',who='peer')==missing and complete(a,key='private',who='other')==missing
        assert complete(a,score=100)[0]==400
        key='repeat-'+c.run_id;grade_before=c.sql('CHECKSUM TABLE grade_reviews,grade_review_history')
        with ThreadPoolExecutor(max_workers=2) as pool:r=list(pool.map(lambda _:complete(a,key=key),range(2)))
        assert [x[0] for x in r]==[200,200] and r[0][1]==r[1][1],r
        receipt=r[0][1];assert receipt['level']==1 and receipt['previous_level']==0
        local=timezone(timedelta(hours=8));completed=parse_time(receipt['completed_at']).astimezone(local)
        assert parse_time(receipt['due_at']).astimezone(local)==(completed+timedelta(days=1)).replace(hour=9,minute=0,second=0,microsecond=0)
        assert c.sql('CHECKSUM TABLE grade_reviews,grade_review_history')==grade_before
        source=req('/api/knowledge-points/'+str(mid))[1]
        for description in ['Alpha','alpha']:
            old_card=card('favorite',mid);payload={k:source[k] for k in ['subject','name','chapter','description','material_ids','assignment_ids']};payload.update(description=description,expected_version=source['version'])
            code,source=req('/api/knowledge-points/'+str(mid),payload,who='teacher',method='PUT');assert code==200;sync();changed=card('favorite',mid)
            assert changed['version']==old_card['version']+1 and changed['body']==description and changed['due_at']==old_card['due_at'] and changed['level']==old_card['level']
        assert complete(a,feedback='again',key=key)[0]==409
        assert a['id'] not in {v['id'] for v in report(status='due')['items']}
        current=card('favorite',mid);assert complete(current)[0]==409
        # A failed history insert must roll back the scheduling update.
        c.sql(f"UPDATE review_cards SET due_at='2020-01-01' WHERE id={current['id']}");current=card('favorite',mid)
        constraint='review_reject_'+c.run_id;c.sql(f"ALTER TABLE review_completions ADD CONSTRAINT {constraint} CHECK(client_key NOT LIKE 'reject-%')");constraints.append(constraint)
        assert complete(current,key='reject-'+c.run_id)[0]==503
        assert card('favorite',mid)==current
        c.sql('ALTER TABLE review_completions DROP CHECK '+constraint);constraints.remove(constraint)
        with ThreadPoolExecutor(max_workers=2) as pool:r=list(pool.map(lambda _:complete(current),range(2)))
        assert sorted(x[0] for x in r)==[200,409],r
        settings=req('/api/review-settings')[1]
        for v in [[1,3,3,14,30],[1,3,7,14],[0,3,7,14,30],[1,3,7,14,366]]:
            assert req('/api/review-settings',{'intervals':v,'expected_version':settings['version']},method='PUT')[0]==400
        original_due=card('favorite',mid)['due_at'];payload={'intervals':[2,4,8,16,32],'expected_version':settings['version']}
        with ThreadPoolExecutor(max_workers=2) as pool:r=list(pool.map(lambda _:req('/api/review-settings',payload,method='PUT'),range(2)))
        assert sorted(x[0] for x in r)==[200,409];assert card('favorite',mid)['due_at']==original_due
        current=card('favorite',mid);c.sql(f"UPDATE review_cards SET due_at='2020-01-01' WHERE id={current['id']}");current=card('favorite',mid)
        code,again=complete(current,'again');assert code==200 and again['level']==0
        due=parse_time(again['due_at']).astimezone(local);assert (due.date()-parse_time(again['completed_at']).astimezone(local).date()).days==2
        current=card('favorite',mid)
        assert req(f'/api/knowledge-points/{mid}/favorite',method='DELETE')[0]==204
        assert mid not in {v['source_id'] for v in cards() if v['source_kind']=='favorite'};assert complete(current)[0]==409
        assert complete(a,key=key)[1]==receipt
        history=req('/api/review-completions')[1]['items'];assert any(h['id']==receipt['id'] for h in history)
        # Immediate mistakes follow AI; teacher correction and new attempts reconcile pending cards.
        c.control(correct=False);aid=c.assignment();sub=c.submit(aid,answer='待纠正的合成回答')[1];c.wait(sub['task_id']);sync();m=card('mistake',sub['submission_id']);code,old=complete(m);assert code==200
        assert req(f'/api/submissions/{sub["submission_id"]}/review',{'score':10,'correct':True,'expected_version':0},who='teacher',method='PUT')[0]==200
        assert m['id'] not in {v['id'] for v in cards()};assert complete({**m,'version':old['card']['version']},key='teacher-corrected')[0]==409
        assert req(f'/api/submissions/{sub["submission_id"]}/review',{'score':0,'correct':False,'expected_version':1},who='teacher',method='PUT')[0]==200
        assert card('mistake',sub['submission_id'])['id']==m['id']
        newer=c.submit(aid,answer='另一份合成回答')[1];assert m['id'] not in {v['id'] for v in cards()};c.wait(newer['task_id']);sync();n=card('mistake',newer['submission_id']);assert n['level']==0 and n['id']!=m['id']
        assert any(h['id']==old['id'] for h in req('/api/review-completions')[1]['items'])
        # Synthetic scale, not real student data.
        ids=[]
        for i in range(100):
            name='review-'+c.run_id+'-scale-'+str(i)
            c.sql(f"INSERT INTO knowledge_points(class_id,subject,name,description,created_by) VALUES({classid},'math','{name}','',{tid})")
        ids=[int(r[0]) for r in c.sql("SELECT id FROM knowledge_points WHERE name LIKE 'review-"+c.run_id+"-scale-%'")];points.extend(ids)
        c.sql('INSERT INTO knowledge_favorites(point_id,student_id) VALUES '+','.join(f'({id},{uid})' for id in ids));sync()
        for _ in range(5):report()
        times=[]
        for _ in range(100):
            t=time.monotonic();report();times.append((time.monotonic()-t)*1000)
        p95=sorted(times)[94];assert p95<=500,p95
        assert req('/api/review-cards?page_size=101')[1]['page_size']==100
        snapshot=c.sql('CHECKSUM TABLE review_cards,review_settings,review_completions')
        for _ in range(2):
            c.compose('restart','api')
            for _ in range(100):
                try:
                    if req('/api/review-cards/count')[0]==200:break
                except Exception:pass
                time.sleep(.2)
            assert c.sql('CHECKSUM TABLE review_cards,review_settings,review_completions')==snapshot
        c.compose('stop','db')
        try:assert req('/api/review-cards/count')[0]==503
        finally:
            c.compose('start','db')
            for _ in range(100):
                try:
                    if req('/api/review-cards/count')[0]==200:break
                except Exception:pass
                time.sleep(.2)
        assert c.sql('CHECKSUM TABLE review_cards,review_settings,review_completions')==snapshot
        return {'sources':'favorite and current effective mistake; corrected/new attempt excluded','feedback':'deterministic China09:00; settings only future','retry':'same receipt, no duplicate; concurrent200/409','failure':'history rollback; DB503/recovery','history':'retained after removal','migrations':'restart2, checksums unchanged','list_p95_ms':round(p95,2)}
    finally:
        for name in constraints:c.sql('ALTER TABLE review_completions DROP CHECK '+name)
        after={int(r[0]) for r in c.sql(f'SELECT id FROM review_cards WHERE student_id={uid}')};newcards=list(after-before)
        if newcards:c.sql('DELETE FROM review_cards WHERE id IN ('+','.join(map(str,newcards))+')')
        for id in points:c.sql('DELETE FROM knowledge_points WHERE id='+str(id))
        if prior:
            raw,version,created,updated=prior[0];c.sql(f"UPDATE review_settings SET intervals_json=CONVERT(UNHEX('{raw}') USING utf8mb4),version={version},created_at='{created}',updated_at='{updated}' WHERE student_id={uid} AND class_id={classid}")
        else:c.sql(f'DELETE FROM review_settings WHERE student_id={uid} AND class_id={classid}')
        c.control()
