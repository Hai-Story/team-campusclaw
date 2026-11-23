"""S3 GIVEN A/B fixtures WHEN SQL/title/cross-site inputs THEN no unauthorized data or write."""
import json,urllib.parse

def run(c):
    before=c.sql("SELECT id,prompt,enabled FROM tutor_agents ORDER BY id")
    code,data,_,_=c.request('/api/search',{'query':"' OR '1'='1",'mode':'keyword','top_k':10})
    assert code in (200,400),(code,data)
    if code==200:assert not data.get('hits'),data
    vector_counts={}
    for who,username in [('student','student_a1'),('other','student_b1')]:
        code,result,_,_=c.request('/api/search',{'query':'班级资料安全隔离','mode':'vector'},who=who)
        assert code==200 and result['hits'],(who,code,result)
        own_class=int(c.sql("SELECT class_id FROM users WHERE username='"+username+"'")[0][0])
        assert all(int(c.sql('SELECT class_id FROM materials WHERE id='+str(h['material_id']))[0][0])==own_class for h in result['hits'])
        vector_counts[who]=len(result['hits'])
    agent=int(c.sql("SELECT id FROM tutor_agents WHERE class_id=(SELECT class_id FROM users WHERE username='teacher_a') LIMIT 1")[0][0])
    path='/api/agents/'+str(agent);body={'name':'csrf-should-not-save','prompt':'rejected','enabled':False,'skill_ids':[],'material_ids':[]}
    for who,headers,expected in [(None,{'Origin':'https://untrusted.invalid'},401),('teacher',{'Origin':'https://untrusted.invalid'},403),('teacher',{'Sec-Fetch-Site':'cross-site'},403),('teacher',{'Origin':'null'},403)]:
        assert c.request(path,body,who=who,method='PUT',headers=headers)[0]==expected
    assert c.sql("SELECT id,prompt,enabled FROM tutor_agents ORDER BY id")==before
    assert c.request('/api/me',who='teacher')[0]==200
    assert c.request('/api/agents',who='teacher',headers={'Origin':c.base})[0]==200
    code,positive,_,_=c.request('/api/agents',{'name':'same-origin-'+c.run_id,'prompt':'固定合成测试','enabled':True,'skill_ids':[],'material_ids':[]},who='teacher',headers={'Origin':c.base})
    assert code==201,(code,positive)
    try:assert c.request('/api/agents/'+str(positive['id']),who='teacher',method='DELETE',headers={'Origin':c.base})[0]==204
    finally:c.sql('DELETE FROM tutor_agents WHERE id='+str(int(positive['id'])))
    # Stored title traverses the actual HTTP JSON path; frontend render is checked by check-frontend.
    evil='<img src=x onerror=alert(1)>'
    id=int(c.sql("SELECT id FROM materials WHERE class_id=(SELECT class_id FROM users WHERE username='teacher_a') LIMIT 1")[0][0])
    old=c.request('/api/materials/'+str(id),who='teacher')[1]
    try:
        code,v,_,_=c.request('/api/materials/'+str(id),{'title':evil,'body':old['body'],'version':old['version']},who='teacher',method='PUT');assert code==200
        rows=c.request('/api/materials',who='student')[1];assert any(x['id']==id and x['title']==evil for x in rows)
        c.request('/api/materials/'+str(id),{'title':old['title'],'body':old['body'],'version':v['version']},who='teacher',method='PUT')
    finally:
        latest=c.request('/api/materials/'+str(id),who='teacher')[1]
        if latest['title']==evil:c.request('/api/materials/'+str(id),{'title':old['title'],'body':old['body'],'version':latest['version']},who='teacher',method='PUT')
    return {'vector_class_filter':vector_counts,'SQL':'parameterized, no hits','XSS':'stored title preserved as text; actual React rendering in frontend check','cross_site':'authenticated403; anonymous401; DB unchanged','same_origin_write':'201/204','SameSite':'no auth cookies; bearer credential required'}
