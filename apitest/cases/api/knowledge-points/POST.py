"""K1/K2/K3: real MySQL authorization, atomic links, version conflicts and favorites."""
from concurrent.futures import ThreadPoolExecutor

def run(c):
    points=[]
    def req(path,body=None,who='teacher',method=None):
        code,data,_,_=c.request(path,body,who,method)
        return code,data
    def create(subject='math',**extra):
        body={'subject':subject,'name':'apitest-'+c.run_id+'-'+str(len(points)),'chapter':'分数','description':'基础概念',**extra}
        code,p=req('/api/knowledge-points',body)
        assert code==201,(code,p)
        points.append(p['id'])
        return p,body
    try:
        c.token('teacher');c.token('student');c.token('peer');c.token('other')
        assert req('/api/knowledge-points',who=None)[0]==401
        assert req('/api/knowledge-points',{'subject':'math','name':'x'},who='student')[0]==403
        math=c.assignment(subject='math');science=c.assignment(subject='science')
        materials=req('/api/materials')[1]
        own_material=materials[0]['id']
        foreign_material=req('/api/materials',who='other')[1][0]['id']
        p,body=create(material_ids=[own_material],assignment_ids=[math])
        assert p['version']==1 and p['material_ids']==[own_material] and p['assignment_ids']==[math] and p['favorite'] is False
        for subject in ['chinese','english','science','morality']:
            item,_=create(subject)
            code,listing=req('/api/knowledge-points?subject='+subject,who='student')
            assert code==200 and all(x['subject']==subject for x in listing['items']) and item['id'] in [x['id'] for x in listing['items']]
        path='/api/knowledge-points/'+str(p['id'])
        foreign=req(path,who='other');missing=req('/api/knowledge-points/9223372036854775807',who='other')
        assert foreign==missing and foreign[0]==404,(foreign,missing)
        assert req('/api/knowledge-points',body)[0]==409
        for changed in [{'material_ids':[own_material,foreign_material]},{'assignment_ids':[science]}]:
            code,_=req(path,{**body,**changed,'expected_version':1},method='PUT')
            assert code==404,code
            assert req(path)[1]==p,'invalid association changed point or version'
        before=int(c.sql('SELECT COUNT(*) FROM knowledge_points')[0][0])
        assert req('/api/knowledge-points',{**body,'name':body['name']+'-bad','material_ids':[foreign_material]})[0]==404
        assert int(c.sql('SELECT COUNT(*) FROM knowledge_points')[0][0])==before
        for invalid in [{'subject':'all'},{'name':' '},{'name':'字'*121},{'material_ids':[0]},{'assignment_ids':[math,math]},{'class_id':2}]:
            assert req('/api/knowledge-points',{**body,**invalid})[0]==400,invalid
        for query in ['page=0','page_size=0','page=1.5','favorites=1','subject=all','subject=math&subject=english','unknown=x']:
            assert req('/api/knowledge-points?'+query)[0]==400,query
        code,listing=req('/api/knowledge-points?page_size=101&favorites=false')
        assert code==200 and listing['page_size']==100
        assert req(path+'/favorite',who='teacher',method='PUT')[0]==403
        for _ in range(2):assert req(path+'/favorite',who='student',method='PUT')[0]==204
        assert req(path,who='student')[1]['favorite'] is True
        assert req(path,who='peer')[1]['favorite'] is False
        assert req(path+'/favorite',who='other',method='PUT')[0]==404
        assert [x['id'] for x in req('/api/knowledge-points?favorites=true',who='student')[1]['items']]==[p['id']]
        for _ in range(2):assert req(path+'/favorite',who='student',method='DELETE')[0]==204
        assert req(path,who='student')[1]['favorite'] is False
        with ThreadPoolExecutor(max_workers=2) as pool:
            outcomes=list(pool.map(lambda name:req(path,{**body,'description':name,'expected_version':1},method='PUT')[0],['first','second']))
        assert sorted(outcomes)==[200,409],outcomes
        assert req(path)[1]['version']==2
        assert req(path,{'expected_version':1},method='DELETE')[0]==409
        assert req(path,{'expected_version':2},method='DELETE')[0]==204
        assert req(path)[0]==404
        assert req('/api/materials/'+str(own_material))[0]==200
        assert req('/api/assignments/'+str(math))[0]==200
        return {'subjects':5,'cross_class':'same 404','invalid_links':'atomic rollback','concurrent_update':sorted(outcomes),'favorites':'isolated/idempotent','original_resources':'preserved'}
    finally:
        if points:c.sql('DELETE FROM knowledge_points WHERE id IN ('+','.join(str(int(x)) for x in points)+')')
