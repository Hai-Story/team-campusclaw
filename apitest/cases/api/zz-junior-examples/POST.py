"""J1/J2: nine-subject original examples, resumable import and privacy."""
import json

def run(c):
    prefix='junior-'+c.run_id;trigger='junior_hw_failure_'+c.run_id;ids=[];cls=None;teacher=prefix+'-teacher';student=prefix+'-student'
    def req(path,body=None,who='junior_teacher',method=None):return c.request(path,body,who,method)[:2]
    try:
        c.sql("INSERT INTO classes(name) VALUES('"+prefix+"');");cls=int(c.sql("SELECT id FROM classes WHERE name='"+prefix+"'")[0][0])
        for role,name,source in [('teacher',teacher,'teacher_a'),('student',student,'student_a1')]:
            c.sql("INSERT INTO users(username,password_hash,role,class_id) SELECT '"+name+"',password_hash,role,"+str(cls)+" FROM users WHERE username='"+source+"'");ids.append(int(c.sql("SELECT id FROM users WHERE username='"+name+"'")[0][0]))
            code,v=req('/api/login',{'username':name,'password':c.env['SEED_TEACHER_PASSWORD' if role=='teacher' else 'SEED_STUDENT_PASSWORD']},who=None);assert code==200;c.tokens['junior_'+role]=v['token']
        # Index dependency failure leaves just the staged source, then resumes exactly once.
        try:c.compose('exec','-T','-e','EMBEDDING_BASE_URL=http://127.0.0.1:1/v1','api','/app/seed-junior','-apply','-teacher',teacher)
        except RuntimeError:pass
        else:raise AssertionError('index failure incorrectly succeeded')
        assert c.sql(f"SELECT status FROM curriculum_seed_items WHERE class_id={cls}")==[['staged']]
        assert c.sql(f'SELECT COUNT(*) FROM knowledge_points WHERE class_id={cls}')[0][0]=='0'
        value=c.compose('exec','-T','api','/app/seed-junior','-apply','-teacher',teacher);assert '"Created":54' in value,value[-300:]
        assert c.sql(f"SELECT COUNT(*) FROM curriculum_seed_items WHERE class_id={cls} AND status='complete'")[0][0]=='54'
        counts={}
        for subject in ['chinese','math','english','physics','chemistry','biology','politics','history','geography']:
            points=req('/api/knowledge-points?subject='+subject+'&page_size=100')[1]['items'];assert len(points)==12
            graph=req('/api/knowledge-graph?subject='+subject)[1];assert len(graph['nodes'])==12 and len(graph['edges'])==6
            bank=req('/api/bank-questions?subject='+subject+'&page_size=100')[1]['items'];assert len(bank)==24
            assert {x['question']['type'] for x in bank}=={'single_choice','multiple_choice','true_false','short_answer'}
            lessons=req('/api/lesson-plans?subject='+subject+'&page_size=100')[1]['items'];assert len(lessons)==6
            for row in bank:
                assert all(r['available'] for r in row['citations'])
                assert req('/api/bank-questions/'+str(row['id']),who='junior_student')[0]==403
                assert req('/api/bank-questions/'+str(row['id']),who='teacher')[0]==404
            lesson=req('/api/lesson-plans/'+str(lessons[0]['id']))[1];ref=lesson['citations'][0]
            text=req('/api/materials/'+str(ref['material_id']))[1]['body'];assert ref['excerpt'] in text
            assert req('/api/lesson-plans/'+str(lesson['id']),who='junior_student')[0]==403
            assert req('/api/lesson-plans/'+str(lesson['id']),who='teacher')==req('/api/lesson-plans/9223372036854775807',who='teacher')
            assert req('/api/knowledge-points/'+str(points[0]['id']),who='student')==req('/api/knowledge-points/9223372036854775807',who='student')
            body=req('/api/knowledge-graph/nodes/'+str(points[0]['id'])+'/resources',who='junior_student')[1];assert body['materials']
            counts[subject]={'points':len(points),'questions':len(bank),'lessons':len(lessons),'edges':len(graph['edges'])}
        # A teacher edit survives repeated imports byte for byte.
        edit={"title":lesson['title'],'body':lesson['body']+'\n\n教师补充：课堂反馈记录。','expected_version':lesson['version']}
        assert req('/api/lesson-plans/'+str(lesson['id']),edit,method='PUT')[0]==200
        saved=req('/api/lesson-plans/'+str(lesson['id']))[1]
        checks=c.sql('CHECKSUM TABLE materials,knowledge_entries,knowledge_chunks,knowledge_points,knowledge_graph_edges,bank_questions,question_drafts,lesson_plans,curriculum_seed_items')
        v=c.compose('exec','-T','api','/app/seed-junior','-apply','-teacher',teacher);assert '"Skipped":54' in v
        assert c.sql('CHECKSUM TABLE materials,knowledge_entries,knowledge_chunks,knowledge_points,knowledge_graph_edges,bank_questions,question_drafts,lesson_plans,curriculum_seed_items')==checks
        assert req('/api/lesson-plans/'+str(lesson['id']))[1]==saved
        # No private answer / plan text is materialized in shared knowledge entries.
        assert c.sql(f"SELECT COUNT(*) FROM knowledge_entries WHERE class_id={cls} AND body_text LIKE '%教师补充：课堂反馈记录%'")[0][0]=='0'
        # U3: committed single + staged private draft survive publication failure.
        c.control()
        c.sql('ALTER TABLE published_paper_questions ADD CONSTRAINT '+trigger+' CHECK(position<2)')
        try:c.compose('exec','-T','api','/app/seed-junior','-apply','-homework','-teacher',teacher)
        except RuntimeError:pass
        else:raise AssertionError('publication failure was ignored')
        finally:c.sql('ALTER TABLE published_paper_questions DROP CHECK '+trigger)
        assert c.sql(f'SELECT COUNT(*) FROM grade_assignments WHERE class_id={cls}')[0][0]=='1'
        assert c.sql(f'SELECT COUNT(*) FROM published_papers WHERE class_id={cls}')[0][0]=='0'
        assert c.sql(f'SELECT COUNT(*) FROM paper_drafts WHERE class_id={cls}')[0][0]=='1'
        result=c.compose('exec','-T','api','/app/seed-junior','-apply','-homework','-teacher',teacher)
        assert '"CreatedSingle":17' in result and '"CreatedPapers":9' in result and '"SkippedSingle":1' in result,result
        for subject in counts:
            bare=req('/api/assignments?subject='+subject+'&page_size=100')[1]['items'];assert len(bare)==2
            papers=req('/api/published-papers?subject='+subject)[1]['items'];assert len(papers)==1 and papers[0]['question_count']==4
            visible=req('/api/published-papers/'+str(papers[0]['id']),who='junior_student')[1]
            assert all('answer' not in q and 'explanation' not in q for q in visible['questions'])
            assert all('expected_answer' not in a for a in req('/api/assignments?subject='+subject,who='junior_student')[1]['items'])
        c.sql(f"UPDATE grade_assignments SET title=CONCAT(title,'（确认稿）') WHERE class_id={cls} AND id=(SELECT n FROM (SELECT MIN(id) n FROM grade_assignments WHERE class_id={cls}) x)")
        tables_checksum='grade_assignments,knowledge_point_assignments,paper_drafts,published_papers,published_paper_questions,curriculum_seed_items'
        before_repeat=c.sql('CHECKSUM TABLE '+tables_checksum)
        result=c.compose('exec','-T','api','/app/seed-junior','-apply','-homework','-teacher',teacher)
        assert '"CreatedSingle":0' in result and '"CreatedPapers":0' in result and '"SkippedSingle":18' in result and '"SkippedPapers":9' in result,result
        assert c.sql('CHECKSUM TABLE '+tables_checksum)==before_repeat
        assert c.sql(f'SELECT COUNT(*) FROM submissions WHERE class_id={cls}')[0][0]=='0'
        assert c.fixture('/stats')['calls']==0
        # A single selected bank item is also a valid published homework container.
        q=req('/api/bank-questions?subject=history')[1]['items'][0]
        code,one=req('/api/paper-drafts',{'subject':'history','title':prefix+'-one','items':[{'question_id':q['id'],'question_version':q['version'],'score':10}],'client_key':prefix+'-one'});assert code==201
        code,one=req('/api/paper-drafts/'+str(one['id'])+'/publish',{'expected_version':1});assert code==201 and one['question_count']==1
        # Exercise every subject-bearing shape and its JSON before replaying migrations.
        phy=req('/api/knowledge-points?subject=physics&page_size=100')[1]['items'][0]
        mid=phy['material_ids'][0]
        c.control(graph_response={'nodes':[{'name':prefix+'候选','chapter':'迁移','description':'候选仅教师可见'}],'edges':[],'citation_indices':[1]})
        code,g=req('/api/knowledge-graph/drafts/generate',{'subject':'physics','material_ids':[mid],'client_key':prefix+'-graph'});assert code==200 and g['draft']
        bank=req('/api/bank-questions?subject=physics')[1]['items'][:2]
        code,p=req('/api/paper-drafts',{'subject':'physics','title':'运动学迁移练习','items':[{'question_id':x['id'],'question_version':x['version'],'score':5} for x in bank],'client_key':prefix+'-paper'});assert code==201
        code,pub=req('/api/paper-drafts/'+str(p['id'])+'/publish',{'expected_version':1});assert code==201
        code,a=req('/api/assignments',{'subject':'physics','title':'运动学迁移测试','question':'20米路程用时10秒，平均速度是多少？','expected_answer':'2米每秒','max_score':10});assert code==201
        c.control(correct=False)
        code,r=c.request('/api/assignments/'+str(a['id'])+'/submissions',{'answer':'1米每秒'},who='junior_student',headers={'Idempotency-Key':prefix})[:2];assert code==202
        import time
        for _ in range(200):
            task=req('/api/tasks/'+r['task_id'],who='junior_student')[1]
            if task.get('status')=='succeeded':break
            time.sleep(.1)
        else:raise AssertionError('migration fixture grading failed')
        assert req('/api/submissions/'+str(r['submission_id'])+'/review',{'score':3,'correct':False,'feedback':'核对公式与单位','expected_version':0},method='PUT')[0]==200
        assert req('/api/walkthroughs',{'assignment_id':a['id'],'client_key':prefix+'-walk'},who='junior_student')[0]==201
        assert req('/api/knowledge-points/'+str(phy['id'])+'/favorite',who='junior_student',method='PUT')[0]==204
        assert req('/api/review-cards/sync',{},who='junior_student')[0]==200
        card=req('/api/review-cards?subject=physics&status=all',who='junior_student')[1]['items'][0]
        assert req('/api/review-cards/'+str(card['id'])+'/complete',{'feedback':'good','expected_version':card['version'],'client_key':prefix+'-complete'},who='junior_student')[0]==200
        tables=['grade_assignments','knowledge_points','lesson_plans','walkthroughs','knowledge_graph_scopes','knowledge_graph_edges','knowledge_graph_drafts','question_drafts','bank_questions','paper_drafts','review_cards','published_papers','review_completions']
        # A row-level backup is sufficient for this disposable class; real runtime uses full backup.
        before={t:c.sql(f'SELECT * FROM {t} WHERE class_id={cls} ORDER BY id') for t in tables}
        sql=[]
        for t in tables:
            if t!='review_completions':sql.append(f"UPDATE {t} SET subject=CASE subject WHEN 'physics' THEN 'science' WHEN 'politics' THEN 'morality' ELSE subject END WHERE class_id={cls}")
        for t,col in [('lesson_plans','input_json'),('knowledge_graph_drafts','input_json'),('question_drafts','input_json'),('review_completions','card_json')]:
            sql.append(f"UPDATE {t} SET {col}=JSON_SET({col},'$.subject',CASE JSON_UNQUOTE(JSON_EXTRACT({col},'$.subject')) WHEN 'physics' THEN 'science' WHEN 'politics' THEN 'morality' ELSE JSON_UNQUOTE(JSON_EXTRACT({col},'$.subject')) END) WHERE class_id={cls}")
        sql += ["DELETE FROM grade_migrations WHERE version='005_nine_subjects'","DELETE FROM curriculum_migrations WHERE version='008_nine_subjects.sql'"]
        c.sql(';'.join(sql))
        # Force a later-table unique-key collision after earlier updates would run.
        c.sql(f"INSERT INTO knowledge_graph_scopes(class_id,subject,created_by) VALUES({cls},'physics',{ids[0]})")
        conflict=int(c.sql(f"SELECT id FROM knowledge_graph_scopes WHERE class_id={cls} AND subject='physics'")[0][0])
        checks=c.sql('CHECKSUM TABLE '+','.join(tables))
        try:c.compose('exec','-T','api','/app/seed-junior','-apply','-teacher',teacher)
        except RuntimeError:pass
        else:raise AssertionError('conflicting classification incorrectly succeeded')
        assert c.sql('CHECKSUM TABLE '+','.join(tables))==checks,'failed migration partly moved rows'
        assert c.sql("SELECT COUNT(*) FROM curriculum_migrations WHERE version='008_nine_subjects.sql'")[0][0]=='0'
        c.sql('DELETE FROM knowledge_graph_scopes WHERE id='+str(conflict))
        for repeat in range(2):
            c.compose('restart','api')
            for _ in range(150):
                try:
                    if req('/api/me')[0]==200:break
                except Exception:pass
                time.sleep(.2)
            else:raise AssertionError('migration restart failed')
            after={t:c.sql(f'SELECT * FROM {t} WHERE class_id={cls} ORDER BY id') for t in tables}
            assert before==after,[t for t in tables if before[t]!=after[t]]
        assert req('/api/tasks/'+r['task_id'],who='junior_student')[1]['final_review']['score']==3
        return {'subjects':counts,'migration':'collision rolled back; 13 table shapes + JSON restored identically on two restarts','resume':'failed index resumed','repeat':'54 skipped, checksums unchanged','privacy':'student403 / other teacher & class404','references':'real indexed excerpts','homework':'18 single + 9 four-question; failure resumed; repeat unchanged; no model calls'}
    finally:
        if c.sql("SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS WHERE CONSTRAINT_SCHEMA=DATABASE() AND CONSTRAINT_NAME='"+trigger+"'")[0][0]!='0':c.sql('ALTER TABLE published_paper_questions DROP CHECK '+trigger)
        if cls:
            c.sql(f'DELETE FROM review_completions WHERE class_id={cls}; DELETE FROM review_cards WHERE class_id={cls}; DELETE FROM walkthroughs WHERE class_id={cls}; DELETE FROM published_papers WHERE class_id={cls}; DELETE FROM paper_drafts WHERE class_id={cls}; DELETE FROM submissions WHERE class_id={cls}; DELETE FROM grade_assignments WHERE class_id={cls}; DELETE FROM knowledge_graph_drafts WHERE class_id={cls}; DELETE FROM bank_questions WHERE class_id={cls}; DELETE FROM question_drafts WHERE class_id={cls}; DELETE FROM lesson_plans WHERE class_id={cls}; DELETE FROM knowledge_graph_edges WHERE class_id={cls}; DELETE FROM knowledge_graph_scopes WHERE class_id={cls}; DELETE FROM knowledge_points WHERE class_id={cls};')
            for row in c.sql(f'SELECT id,version FROM materials WHERE class_id={cls}'):
                assert req('/api/materials/'+row[0],{'version':int(row[1])},method='DELETE')[0]==204
            c.sql(f'DELETE FROM tutor_agents WHERE class_id={cls}; DELETE FROM tutor_skills WHERE class_id={cls}; DELETE FROM tutor_app_templates WHERE class_id={cls}; DELETE FROM users WHERE class_id={cls}; DELETE FROM classes WHERE id={cls};')
