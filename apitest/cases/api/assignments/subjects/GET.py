def run(c):
    # A11-S1/S2: five concrete subjects, invalid filters, and class isolation.
    subjects=('chinese','math','english','science','morality')
    created={subject:c.assignment(subject=subject) for subject in subjects}
    for subject,aid in created.items():
        code,data,_,_=c.request('/api/assignments?subject='+subject+'&page_size=100')
        assert code==200 and aid in {x['id'] for x in data['items']}
        assert all(x['subject']==subject for x in data['items'])
        foreign=c.request('/api/assignments?subject='+subject+'&page_size=100',who='other')[1]['items']
        assert not set(created.values()) & {x['id'] for x in foreign}
    default=c.request('/api/assignments?page_size=100')[1]['items']
    assert created['math'] in {x['id'] for x in default} and all(x['subject']=='math' for x in default)
    before=c.sql('SELECT COUNT(*) FROM grade_assignments;')
    for invalid in ('all','physics','chemistry','labor','other',''):
        assert c.request('/api/assignments',{'subject':invalid,'title':'x','question':'x','expected_answer':'x','max_score':10},who='teacher')[0]==400
    for invalid in ('all','physics','chemistry','labor','other',''):
        assert c.request('/api/assignments?subject='+invalid)[0]==400
        assert c.request('/api/mistakes?subject='+invalid)[0]==400
    assert c.sql('SELECT COUNT(*) FROM grade_assignments;')==before
    for _ in range(2):c.compose('exec','-T','worker','/app/grade-worker','-migrate')
    assert c.sql("SELECT COUNT(*) FROM grade_assignments WHERE subject IN ('physics','chemistry');")[0][0]=='0'
    assert c.sql("SELECT COUNT(*) FROM grade_migrations WHERE version='003_unified_subjects';")[0][0]=='1'
    return {'subjects':list(subjects),'all_filter':'rejected400','default_subject':'math','foreign_class':'excluded','invalid_writes':0,'legacy_subjects':0,'migration_repeats':2}
