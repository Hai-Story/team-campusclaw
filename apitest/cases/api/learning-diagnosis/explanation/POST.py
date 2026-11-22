"""D3: authorized aggregate explanation, truthful numeric output, retries/cancel and staleness."""
from concurrent.futures import ThreadPoolExecutor
import http.client,json,time,urllib.parse

def run(c):
    uid=None
    def request(body,who='student'):return c.request('/api/learning-diagnosis/explanation',body,who=who,timeout=130)[:2]
    def report(who='student'):
        code,data,_,_=c.request('/api/learning-diagnosis?subject=morality',who=who);assert code==200;return data
    def wait_calls():
        for _ in range(60):
            if c.fixture('/stats')['calls']:return
            time.sleep(.05)
        raise AssertionError('Model did not start')
    def review(sid,version,score):
        code,data,_,_=c.request(f'/api/submissions/{sid}/review',{'score':score,'correct':False,'expected_version':version,'error_tags':['reading']},who='teacher',method='PUT');assert code==200;return data
    try:
        username='apitest_empty_'+c.run_id
        c.sql("INSERT INTO users(username,password_hash,role,class_id) SELECT '"+username+"',password_hash,'student',class_id FROM users WHERE username='student_a1'")
        uid=int(c.sql("SELECT id FROM users WHERE username='"+username+"'")[0][0])
        code,login,_,_=c.request('/api/login',{'username':username,'password':c.env['SEED_STUDENT_PASSWORD']},who=None);assert code==200;c.tokens['diag_empty']=login['token']
        empty=report('diag_empty');assert empty['samples']==0
        c.control();code,data=request({'subject':'morality','days':30,'fingerprint':empty['fingerprint']},'diag_empty');assert code==200 and data['status']=='insufficient_data' and c.fixture('/stats')['calls']==0
        aid=c.assignment(subject='morality');c.control(correct=False);r=c.submit(aid,answer='合成作答')[1];c.wait(r['task_id']);sid=r['submission_id'];review(sid,0,4)
        v=report();body={'subject':'morality','days':30,'fingerprint':v['fingerprint']};c.control()
        assert request(body,who=None)[0]==401
        for patch in [{'class_id':1},{'student_id':uid},{'days':0},{'subject':'all'},{'fingerprint':'bad'},{'fingerprint':'F'*64}]:assert request({**body,**patch})[0]==400,patch
        assert request({**body,'fingerprint':'0'*64})[0]==409
        assert c.fixture('/stats')['calls']==0
        code,data=request(body);assert code==200 and data['status']=='generated' and data['fingerprint']==body['fingerprint']
        for settings in [{'diagnosis_raw':'not-json'},{'diagnosis_response':{'explanation':' ','score':10}},{'diagnosis_response':{'explanation':'有999999份样本。'}},{'diagnosis_response':{'explanation':'请参考[999]。'}}]:
            c.control(**settings);assert request(body)[0]==503,settings
        c.control(statuses=[503,200]);assert request(body)[0]==200 and c.fixture('/stats')['calls']==2
        c.control(statuses=[401]);assert request(body)[0]==503 and c.fixture('/stats')['calls']==1
        c.control(hold_diagnosis=True)
        with ThreadPoolExecutor(max_workers=1) as pool:
            pending=pool.submit(request,body);wait_calls();assert request(body)[0]==429;c.fixture('/release',{});assert pending.result()[0]==200
        c.control(hold_diagnosis=True)
        with ThreadPoolExecutor(max_workers=1) as pool:
            pending=pool.submit(request,body);wait_calls();review(sid,1,5);c.fixture('/release',{});assert pending.result()[0]==409
        body['fingerprint']=report()['fingerprint'];snapshot=c.sql(f'SELECT version,score,error_tags FROM grade_reviews WHERE submission_id={sid}')
        c.control(hold_diagnosis=True)
        target=urllib.parse.urlsplit(c.base);connection=http.client.HTTPConnection(target.hostname,target.port)
        connection.request('POST','/api/learning-diagnosis/explanation',body=json.dumps(body),headers={'Authorization':'Bearer '+c.token('student'),'Content-Type':'application/json'});wait_calls();connection.close()
        for _ in range(60):
            if c.fixture('/stats')['cancelled']>0:break
            time.sleep(.05)
        assert c.fixture('/stats')['cancelled']==1 and c.sql(f'SELECT version,score,error_tags FROM grade_reviews WHERE submission_id={sid}')==snapshot
        c.control()
        return {'no_samples':'no model call','scope':'fixed own report; no identity injection','invalid_output':503,'retry_calls':2,'parallel':429,'changed_review':409,'cancel':'gateway cancelled; grades unchanged'}
    finally:
        c.control()
        if uid:
            c.request('/api/logout',{},who='diag_empty');c.sql('DELETE FROM users WHERE id='+str(uid))
