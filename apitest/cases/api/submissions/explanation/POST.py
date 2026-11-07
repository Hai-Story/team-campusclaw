from concurrent.futures import ThreadPoolExecutor
import http.client,json,time,urllib.parse

def stats(c):
    return json.loads(c.compose('exec','-T','gateway','node','--input-type=module','-e',"console.log(await(await fetch('http://127.0.0.1:8089/stats')).text())"))

def started(c):
    end=time.monotonic()+3
    while time.monotonic()<end:
        if stats(c)['calls']:return
        time.sleep(.03)
    raise AssertionError('model request did not start')

def release(c):
    c.compose('exec','-T','gateway','node','--input-type=module','-e',"await fetch('http://127.0.0.1:8089/release',{method:'POST',body:'{}'})")

def run(c):
    # A13-S1/S2: ownership and effective judgment gate every model request.
    c.control(correct=False);aid=c.assignment();receipt=c.submit(aid)[1];c.wait(receipt['task_id']);sid=receipt['submission_id']
    path=f'/api/submissions/{sid}/explanation'
    for who in ['student','peer','other','teacher']:c.token(who)
    c.control()
    for who,expected in [(None,401),('teacher',403),('peer',404),('other',404)]:assert c.request(path,{},who=who)[0]==expected
    assert c.request('/api/submissions/99999999/explanation',{})[0]==404
    for body in [{'question':'forged'},{'answer':'2'},{'class_id':1},[]]:assert c.request(path,body)[0]==400
    assert stats(c)['calls']==0
    before=c.sql(f'SELECT answer,updated_at FROM submissions WHERE id={sid};')
    code,data,_,_=c.request(path,{});assert code==200 and set(data)=={'explanation'} and data['explanation']
    assert c.sql(f'SELECT answer,updated_at FROM submissions WHERE id={sid};')==before
    for response in [{'explanation':'结果为2。'},{'explanation':'回想定义。','score':10},{'explanation':''}]:
        c.control(explanation_response=response);assert c.request(path,{})[0]==503
    c.control(statuses=[503]);assert c.request(path,{})[0]==503
    assert stats(c)['calls']==1
    c.control(hold_explanation=True)
    with ThreadPoolExecutor(max_workers=1) as pool:
        first=pool.submit(c.request,path,{});started(c)
        second=c.request(path,{});assert second[0]==429 and second[3].get('Retry-After')=='2'
        release(c)
        assert first.result()[0]==200
    assert stats(c)['calls']==1
    c.control(hold_explanation=True)
    with ThreadPoolExecutor(max_workers=1) as pool:
        pending=pool.submit(c.request,path,{});started(c)
        assert c.request(f'/api/submissions/{sid}/review',{'score':5,'correct':True,'expected_version':0},who='teacher',method='PUT')[0]==200
        release(c)
        assert pending.result()[0]==409
    c.control();assert c.request(path,{})[0]==404 and stats(c)['calls']==0
    assert c.request(f'/api/submissions/{sid}/review',{'score':0,'correct':False,'expected_version':1},who='teacher',method='PUT')[0]==200
    c.control(hold_explanation=True)
    target=urllib.parse.urlsplit(c.base);connection=http.client.HTTPConnection(target.hostname,target.port)
    connection.request('POST',path,body='{}',headers={'Authorization':'Bearer '+c.token('student'),'Content-Type':'application/json'})
    started(c);connection.close()
    end=time.monotonic()+3
    while time.monotonic()<end and stats(c)['cancelled']==0:time.sleep(.03)
    assert stats(c)['cancelled']==1
    c.control(hold_explanation=True)
    with ThreadPoolExecutor(max_workers=1) as pool:
        pending=pool.submit(c.request,path,{});started(c);new=c.submit(aid,answer='3')[1]
        release(c)
        assert pending.result()[0]==409
    assert c.request(path,{})[0]==404
    c.wait(new['task_id']);c.control()
    return {'ownership':'401/403/404, no model calls','valid':200,'invalid_output':503,'concurrent':[200,429],'teacher_changed':409,'new_attempt':409,'cancelled':1,'submission':'unchanged'}
