"""Isolated API fixtures and administration; never operates against a production target."""
from pathlib import Path
import json,subprocess,time,urllib.request,urllib.error,uuid

ROOT=Path(__file__).resolve().parents[1]

class Context:
    def __init__(self):
        self.env=dict(line.split('=',1) for line in (ROOT/'.env').read_text().splitlines() if line and not line.startswith('#') and '=' in line)
        if self.env.get('COMPOSE_PROJECT_NAME')!='campusclaw-team-async' or self.env.get('CHAT_BASE_URL')!='http://gateway:8089/v1':
            raise RuntimeError('仅允许在dev_env.py建立的隔离验证环境运行')
        self.base='http://127.0.0.1:'+self.env['WEB_PORT'];self.tokens={};self.assignments=[];self.run_id=uuid.uuid4().hex[:12]
    def compose(self,*args,input=None):
        result=subprocess.run(['docker','compose',*args],input=input,capture_output=True,text=True,cwd=ROOT)
        if result.returncode:
            detail=result.stderr[-1200:]
            for key,value in self.env.items():
                if len(value)>8 and any(word in key for word in ('PASSWORD','SECRET','TOKEN','API_KEY','ENCRYPTION_KEY','AMQP_URL')):detail=detail.replace(value,'[REDACTED]')
            raise RuntimeError('Compose操作失败: '+args[0]+' (退出'+str(result.returncode)+'): '+detail)
        return result.stdout.strip()
    def sql(self,query):
        result=self.compose('exec','-T','db','sh','-c','MYSQL_PWD="$MYSQL_PASSWORD" exec mysql --default-character-set=utf8mb4 --batch --raw --skip-column-names -u "$MYSQL_USER" "$MYSQL_DATABASE"',input=query)
        return [line.split('\t') for line in result.splitlines() if line]
    def request(self,path,body=None,who='student',method=None,headers=None):
        h={'Content-Type':'application/json',**(headers or {})}
        if who:h['Authorization']='Bearer '+self.token(who)
        req=urllib.request.Request(self.base+path,data=None if body is None else json.dumps(body,ensure_ascii=False).encode(),headers=h,method=method or ('POST' if body is not None else 'GET'))
        start=time.monotonic()
        try:
            with urllib.request.urlopen(req,timeout=10) as response:code=response.status;raw=response.read();response_headers=dict(response.headers)
        except urllib.error.HTTPError as error:code=error.code;raw=error.read();response_headers=dict(error.headers)
        try:data=json.loads(raw) if raw else None
        except ValueError:data=raw.decode(errors='replace')
        return code,data,time.monotonic()-start,response_headers
    def token(self,who):
        if who not in self.tokens:
            name={'teacher':'teacher_a','student':'student_a1','peer':'student_a2','other':'student_b1'}[who]
            password=self.env['SEED_TEACHER_PASSWORD' if who=='teacher' else 'SEED_STUDENT_PASSWORD']
            code,data,_,_=self.request('/api/login',{'username':name,'password':password},who=None)
            assert code==200,'预置账号登录失败: '+name
            self.tokens[who]=data['token']
        return self.tokens[who]
    def assignment(self,title=None):
        code,data,_,_=self.request('/api/assignments',{'title':title or 'apitest-'+self.run_id,'question':'1+1等于多少？','expected_answer':'2','max_score':10},who='teacher')
        assert code==201,('assignment',code);self.assignments.append(data['id']);return data['id']
    def submit(self,assignment,key=None,answer='2'):
        return self.request(f'/api/assignments/{assignment}/submissions',{'answer':answer},headers={'Idempotency-Key':key or uuid.uuid4().hex})
    def task(self,id):return self.request('/api/tasks/'+id)[1]
    def wait(self,id,statuses=('succeeded','failed'),seconds=60):
        deadline=time.monotonic()+seconds
        while time.monotonic()<deadline:
            task=self.task(id)
            if task.get('status') in statuses:return task
            time.sleep(.15)
        raise AssertionError('任务未在时限内到达预期状态: '+id)
    def control(self,**settings):
        code="const r=await fetch('http://127.0.0.1:8089/control',{method:'POST',headers:{'Content-Type':'application/json'},body:process.argv[1]});if(!r.ok)process.exit(1)"
        self.compose('exec','-T','gateway','node','--input-type=module','-e',code,json.dumps(settings))
    def cleanup(self):
        if self.assignments:
            ids=','.join(str(int(x)) for x in self.assignments)
            self.sql(f'DELETE FROM submissions WHERE assignment_id IN ({ids}); DELETE FROM grade_assignments WHERE id IN ({ids});')
        for who in list(self.tokens):
            try:self.request('/api/logout',{},who=who)
            except Exception:pass
