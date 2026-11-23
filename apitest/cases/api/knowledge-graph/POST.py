"""G1-G3: formal graph isolation, review-only mastery and atomic candidate approval."""
import json,time,uuid,http.client
from concurrent.futures import ThreadPoolExecutor

def run(c):
    prefix='graph-'+c.run_id;points=set();drafts=[];empty=None;teacher2=None;token2=None;foreign=None
    def req(path,body=None,who='teacher',method=None):
        if who=='teacher2':return c.request(path,body,who=None,method=method,headers={'Authorization':'Bearer '+token2})[:2]
        return c.request(path,body,who,method,timeout=130 if path.endswith('/generate') else 10)[:2]
    def point(name,**extra):
        code,v=req('/api/knowledge-points',{'subject':'math','name':prefix+'-'+name,'chapter':'图谱','description':'合成节点','material_ids':[],'assignment_ids':[],**extra});assert code==201,(code,v);points.add(v['id']);return v
    def edge(a,b,kind='prerequisite'):return req('/api/knowledge-graph/edges',{'from_id':a,'to_id':b,'kind':kind})
    def graph(who='teacher'):return req('/api/knowledge-graph',who=who)[1]
    def resource(id,who='student'):return req('/api/knowledge-graph/nodes/'+str(id)+'/resources',who=who)[1]
    def generated(nodes,edges=None,key=None):
        c.control(graph_response={'nodes':nodes,'edges':edges or [],'citation_indices':[1]});body={**base,'client_key':key or prefix+'-'+uuid.uuid4().hex[:8]};code,v=req('/api/knowledge-graph/drafts/generate',body);assert code==200 and v['draft'],(code,v);drafts.append(v['draft']['id']);return v['draft'],body
    def gate():return c.fixture('/stats')
    def release():return c.fixture('/release',{})
    try:
        c.control();c.token('teacher');c.token('student');c.token('other');c.token('peer')
        mid=req('/api/materials')[1][0]['id'];base={'subject':'math','material_ids':[mid],'client_key':prefix+'-same'}
        aid=c.assignment();A=point('A',material_ids=[mid],assignment_ids=[aid]);B=point('B');C=point('C');D=point('D');E=point('E')
        c.sql("INSERT INTO knowledge_points(class_id,subject,name,description,created_by) SELECT class_id,'math','"+prefix+"-foreign','',id FROM users WHERE username='student_b1'");foreign=int(c.sql("SELECT id FROM knowledge_points WHERE name='"+prefix+"-foreign'")[0][0])
        assert req('/api/knowledge-graph',who=None)[0]==401
        assert req('/api/knowledge-graph?subject=all')[0]==400
        assert req('/api/knowledge-graph/edges',{'from_id':A['id'],'to_id':B['id'],'kind':'prerequisite'},who='student')[0]==403
        assert edge(A['id'],foreign)[0]==404 and edge(A['id'],A['id'])[0]==400
        assert edge(A['id'],B['id'])[0]==201;assert edge(B['id'],C['id'])[0]==201;before=len(graph()['edges']);assert edge(C['id'],A['id'])[0]==409 and len(graph()['edges'])==before
        first=edge(D['id'],E['id'],'related');second=edge(E['id'],D['id'],'related');assert first[1]['id']==second[1]['id']
        with ThreadPoolExecutor(max_workers=2) as pool:codes=list(pool.map(lambda pair:edge(*pair)[0],[(D['id'],E['id']),(E['id'],D['id'])]))
        assert sorted(codes)==[201,409],codes
        edgeid=first[1]['id'];assert req('/api/knowledge-graph/edges/'+str(edgeid),{'expected_version':2},method='DELETE')[0]==409
        assert req('/api/knowledge-graph/edges/'+str(edgeid),{'expected_version':1},method='DELETE')[0]==204
        assert req('/api/knowledge-graph/nodes/'+str(A['id'])+'/resources',who='other')==req('/api/knowledge-graph/nodes/9223372036854775807/resources',who='other')
        assert req('/api/knowledge-points/'+str(A['id']),{'subject':'physics','name':A['name'],'chapter':'','description':'','material_ids':[],'assignment_ids':[],'expected_version':1},method='PUT')[0]==409
        r=resource(B['id']);assert any(n['id']==A['id'] for n in r['recommended_points']) and any(m['id']==mid for m in r['review_materials'])
        c.control(correct=False);receipt=c.submit(aid,answer='不完整的合成作答')[1];assert c.wait(receipt['task_id'])['status']=='succeeded'
        r=resource(A['id']);assert r['reviewed']==0 and not r['weak'] and len(r['mistakes'])==1
        sid=receipt['submission_id']
        for version,correct in [(0,False),(1,True)]:
            assert req('/api/submissions/'+str(sid)+'/review',{'score':10 if correct else 0,'correct':correct,'feedback':'合成终评','expected_version':version},method='PUT')[0]==200
            r=resource(A['id']);assert r['reviewed']==1 and r['incorrect']==int(not correct) and r['weak']==(not correct)
        assert resource(A['id'],'peer')['reviewed']==0 and resource(A['id'],'peer')['mistakes']==[]
        assert resource(A['id'],'teacher')['reviewed']==1 and resource(A['id'],'teacher')['mistakes']==[]
        later=c.submit(aid,answer='新的不完整作答')[1];assert c.wait(later['task_id'])['status']=='succeeded'
        assert resource(A['id'])['reviewed']==0
        assert req('/api/submissions/'+str(later['submission_id'])+'/review',{'score':0,'correct':False,'feedback':'最新终评','expected_version':0},method='PUT')[0]==200
        assert resource(A['id'])['reviewed']==1 and resource(A['id'])['incorrect']==1
        candidate,body=generated([{'name':prefix+'-candidate','chapter':'章节','description':'候选说明'}]);path='/api/knowledge-graph/drafts/'+str(candidate['id'])
        assert not any(n['name']==prefix+'-candidate' for n in graph('student')['nodes'])
        c.control();assert req('/api/knowledge-graph/drafts/generate',body)[1]['draft']['id']==candidate['id'] and gate()['calls']==0
        assert req('/api/knowledge-graph/drafts/generate',{**body,'subject':'physics'})[0]==409
        assert req(path,who='student')[0]==403
        username='graph_teacher_'+c.run_id;c.sql("INSERT INTO users(username,password_hash,role,class_id) SELECT '"+username+"',password_hash,'teacher',class_id FROM users WHERE username='teacher_a'");teacher2=int(c.sql("SELECT id FROM users WHERE username='"+username+"'")[0][0]);token2=req('/api/login',{'username':username,'password':c.env['SEED_TEACHER_PASSWORD']},who=None)[1]['token'];assert req(path,who='teacher2')==req('/api/knowledge-graph/drafts/9223372036854775807',who='teacher2')
        edit={'nodes':candidate['nodes'],'edges':[],'citation_indices':[1],'expected_version':1}
        with ThreadPoolExecutor(max_workers=2) as pool:edit_codes=list(pool.map(lambda _:req(path,edit,method='PUT')[0],range(2)))
        assert sorted(edit_codes)==[200,409]
        ref=candidate['citations'][0];chunk=ref['chunk_id'];original=c.sql('SELECT HEX(chunk_text) FROM knowledge_chunks WHERE id='+str(chunk))[0][0]
        try:
            c.sql("UPDATE knowledge_chunks SET chunk_text=CONCAT(chunk_text,' changed') WHERE id="+str(chunk));assert req(path+'/approve',{'expected_version':2})[0]==409
            listed=next(v for v in req('/api/knowledge-graph/drafts?page_size=100')[1]['items'] if v['id']==candidate['id']);assert any(not ref['available'] and not ref['excerpt'] for ref in listed['citations'])
            assert not any(n['name']==prefix+'-candidate' for n in graph()['nodes'])
        finally:c.sql("UPDATE knowledge_chunks SET chunk_text=UNHEX('"+original+"') WHERE id="+str(chunk))
        cyclic={'nodes':[{'name':prefix+'-candidate','chapter':'','description':''}],'edges':[{'from':C['name'],'to':A['name'],'kind':'prerequisite'}],'citation_indices':[1],'expected_version':2}
        assert req(path,cyclic,method='PUT')[0]==200;assert req(path+'/approve',{'expected_version':3})[0]==409;assert not any(n['name']==prefix+'-candidate' for n in graph()['nodes'])
        assert req(path,{**cyclic,'edges':[],'expected_version':3},method='PUT')[0]==200
        code,approved=req(path+'/approve',{'expected_version':4});assert code==200 and approved['status']=='approved';points.update(approved['point_ids']);assert req(path+'/approve',{'expected_version':4})[1]['point_ids']==approved['point_ids'];assert req(path,edit,method='PUT')[0]==409
        assert any(m['id']==mid for m in resource(approved['point_ids'][0])['materials'])
        # A formerly existing directory name cannot silently become a fresh node.
        saved,_=generated([{'name':A['name'],'chapter':'','description':''}]);sp='/api/knowledge-graph/drafts/'+str(saved['id']);current=req('/api/knowledge-points/'+str(A['id']))[1]
        payload={k:current[k] for k in ['subject','name','chapter','description','material_ids','assignment_ids']};payload['expected_version']=current['version'];assert req('/api/knowledge-points/'+str(A['id']),{**payload,'name':A['name']+'-renamed'},method='PUT')[0]==200
        assert req(sp+'/approve',{'expected_version':1})[0]==409
        assert req('/api/knowledge-points/'+str(A['id']),{**payload,'expected_version':current['version']+1},method='PUT')[0]==200
        # Capacity is a service boundary; fill only disposable rows in this isolated DB.
        classid,userid=map(int,c.sql("SELECT class_id,id FROM users WHERE username='teacher_a'")[0]);count=int(c.sql("SELECT COUNT(*) FROM knowledge_points WHERE class_id="+str(classid)+" AND subject='math'")[0][0]);needed=500-count
        if needed>0:
            values=','.join('('+str(classid)+",'math','"+prefix+'-limit-'+str(i)+"','',"+str(userid)+')' for i in range(needed));c.sql('INSERT INTO knowledge_points(class_id,subject,name,description,created_by) VALUES '+values)
        assert len(graph()['nodes'])==500
        code,_=req('/api/knowledge-points',{'subject':'math','name':prefix+'-over-limit','chapter':'','description':'','material_ids':[],'assignment_ids':[]});assert code==409
        capacity_ids=[int(r[0]) for r in c.sql("SELECT id FROM knowledge_points WHERE name LIKE '"+prefix+"-limit-%' ORDER BY id LIMIT 65")]
        extra=2000-len(graph()['edges']);pairs=[(a,b) for i,a in enumerate(capacity_ids) for b in capacity_ids[i+1:]][:extra]
        assert len(pairs)==extra
        values=','.join('('+str(classid)+",'math',"+str(a)+','+str(b)+",'related',"+str(userid)+')' for a,b in pairs)
        c.sql('INSERT INTO knowledge_graph_edges(class_id,subject,from_id,to_id,kind,created_by) VALUES '+values)
        assert len(graph()['edges'])==2000 and edge(capacity_ids[-2],capacity_ids[-1],'related')[0]==409
        assert edge(capacity_ids[0],capacity_ids[1],'related')[0]==201
        c.sql("DELETE FROM knowledge_points WHERE name LIKE '"+prefix+"-limit-%'")
        empty_title=prefix+'-empty';c.sql("INSERT INTO materials(class_id,title,file_path,uploaded_by) VALUES("+str(classid)+",'"+empty_title+"','isolated-no-file',"+str(userid)+')');empty=int(c.sql("SELECT id FROM materials WHERE title='"+empty_title+"'")[0][0]);c.control();code,nohit=req('/api/knowledge-graph/drafts/generate',{**base,'material_ids':[empty],'client_key':prefix+'-empty'});assert code==200 and nohit['draft'] is None and gate()['calls']==0
        for settings in [{'graph_raw':'not json'},{'graph_response':{'nodes':[{'name':'坏节点','chapter':'','description':'伪造[999]'}],'edges':[],'citation_indices':[1]}},{'graph_response':{'nodes':[{'name':'坏节点','chapter':'','description':''}],'edges':[],'citation_indices':[999]}}]:
            c.control(**settings);assert req('/api/knowledge-graph/drafts/generate',{**base,'client_key':prefix+'-bad-'+uuid.uuid4().hex[:5]})[0]==503
        fixture={'nodes':[{'name':prefix+'-pending','chapter':'','description':''}],'edges':[],'citation_indices':[1]}
        def wait_call():
            for _ in range(40):
                if gate()['calls']>0:return
                time.sleep(.05)
            raise AssertionError('graph call never started')
        c.control(hold_graph=True,graph_response=fixture)
        with ThreadPoolExecutor(max_workers=1) as pool:
            pending=pool.submit(req,'/api/knowledge-graph/drafts/generate',{**base,'client_key':prefix+'-held'});wait_call();assert req('/api/knowledge-graph/drafts/generate',{**base,'client_key':prefix+'-parallel'})[0]==429;release();assert pending.result()[0]==200
        c.control(hold_graph=True,graph_response=fixture)
        connection=http.client.HTTPConnection('127.0.0.1',18081,timeout=5);connection.request('POST','/api/knowledge-graph/drafts/generate',json.dumps({**base,'client_key':prefix+'-cancel'}),{'Content-Type':'application/json','Authorization':'Bearer '+c.token('teacher')});wait_call();connection.close()
        for _ in range(40):
            if gate()['cancelled']>0:break
            time.sleep(.05)
        else:raise AssertionError('cancel did not reach graph model')
        release();assert c.sql("SELECT COUNT(*) FROM knowledge_graph_drafts WHERE client_key='"+prefix+"-cancel'")[0][0]=='0'
        c.control()
        timings=[c.request('/api/knowledge-graph',who='student')[2]*1000 for _ in range(100)];p95=sorted(timings)[94];assert p95<=500
        return {'cycles':'three-node and concurrent reverse rejected','capacity':'500 nodes/2000 edges','private':'student403/other-teacher same404','approval':'source/cycle/version atomic; repeated no duplicates','mastery':'AI excluded; final versions counted once; correction reflected','resources':'prerequisite material; own immediate mistakes','list_p95_ms':round(p95,2)}
    finally:
        c.control();c.sql("DELETE FROM knowledge_graph_drafts WHERE client_key LIKE '"+prefix+"%'; DELETE FROM knowledge_points WHERE name LIKE '"+prefix+"-limit-%'")
        if points:c.sql('DELETE FROM knowledge_points WHERE id IN ('+','.join(map(str,points))+')')
        if foreign:c.sql('DELETE FROM knowledge_points WHERE id='+str(foreign))
        if empty:c.sql('DELETE FROM materials WHERE id='+str(empty))
        if teacher2:c.sql('DELETE FROM sessions WHERE user_id='+str(teacher2)+'; DELETE FROM users WHERE id='+str(teacher2))
