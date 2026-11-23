"""Isolated API fixtures and administration; never operates against a production target."""
from pathlib import Path
import atexit,json,os,select,subprocess,threading,time,urllib.request,urllib.error,uuid

ROOT=Path(__file__).resolve().parents[1]

class Context:
    def __init__(self):
        self.env_file=ROOT/os.environ.get('CAMPUSCLAW_ENV_FILE','.env.test')
        self.env=dict(line.split('=',1) for line in self.env_file.read_text().splitlines() if line and not line.startswith('#') and '=' in line)
        if self.env.get('COMPOSE_PROJECT_NAME')!='campusclaw-team-test' or self.env.get('CHAT_BASE_URL')!='http://gateway:8089/v1' or self.env.get('WEB_PORT')!='18081':
            raise RuntimeError('仅允许在dev_env.py建立的隔离验证环境运行')
        self.base='http://127.0.0.1:'+self.env['WEB_PORT'];self.tokens={};self.assignments=[];self.run_id=uuid.uuid4().hex[:12]
        self._sql_channels={};self._sql_lock=threading.Lock();atexit.register(self.close_sql)
    def compose(self,*args,input=None):
        if args and (args[0]=='down' or args[0] in ('stop','restart','start','up') and 'db' in args):self.close_sql()
        result=subprocess.run(['docker','compose','--env-file',str(self.env_file),*args],input=input,capture_output=True,text=True,cwd=ROOT,timeout=180)
        if result.returncode:
            detail=result.stderr[-1200:]
            for key,value in self.env.items():
                if len(value)>8 and any(word in key for word in ('PASSWORD','SECRET','TOKEN','API_KEY','ENCRYPTION_KEY','AMQP_URL')):detail=detail.replace(value,'[REDACTED]')
            raise RuntimeError('Compose操作失败: '+args[0]+' (退出'+str(result.returncode)+'): '+detail)
        return result.stdout.strip()
    def sql(self,query):
        # Reuse only the command channel. Each query still launches a fresh mysql
        # client, preserving session/transaction isolation and exact error codes.
        key=threading.get_ident()
        with self._sql_lock:
            p=self._sql_channels.get(key)
            if p is None or p.poll() is not None:
                p=subprocess.Popen(['docker','compose','--env-file',str(self.env_file),'exec','-T','db','sh'],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,cwd=ROOT,bufsize=0)
                self._sql_channels[key]=p
        marker='SQL_END_'+uuid.uuid4().hex
        while marker in query:marker='SQL_END_'+uuid.uuid4().hex
        script='MYSQL_PWD="$MYSQL_PASSWORD" mysql --default-character-set=utf8mb4 --batch --raw --skip-column-names -u "$MYSQL_USER" "$MYSQL_DATABASE" <<\''+marker+'\'\n'+query+'\n'+marker+'\nstatus=$?\nprintf "\\n'+marker+':%s\\n" "$status"\n'
        end=('\n'+marker+':').encode();data=b'';deadline=time.monotonic()+180
        try:
            p.stdin.write(script.encode());p.stdin.flush()
            while True:
                remaining=deadline-time.monotonic()
                if remaining<=0 or not select.select([p.stdout],[],[],max(0,remaining))[0]:raise TimeoutError('测试数据库命令超时')
                chunk=os.read(p.stdout.fileno(),65536)
                if not chunk:raise RuntimeError('测试数据库命令通道已关闭，请恢复测试数据库后重试')
                data+=chunk
                if end in data and data.endswith(b'\n'):
                    body,code=data.rsplit(end,1);status=int(code.strip());result=body.decode(errors='replace').strip()
                    if status:
                        for name,value in self.env.items():
                            if len(value)>8 and any(w in name for w in ('PASSWORD','SECRET','TOKEN','API_KEY','ENCRYPTION_KEY','AMQP_URL')):result=result.replace(value,'[REDACTED]')
                        raise RuntimeError('测试数据库操作失败 (退出'+str(status)+'): '+result[-1200:])
                    return [line.split('\t') for line in result.splitlines() if line]
        except (BrokenPipeError,TimeoutError):
            with self._sql_lock:self._sql_channels.pop(key,None)
            p.terminate()
            raise
    def close_sql(self):
        with self._sql_lock:
            channels=list(self._sql_channels.values());self._sql_channels.clear()
        for p in channels:
            try:p.stdin.close();p.wait(timeout=2)
            except (OSError,subprocess.TimeoutExpired):p.terminate()
    def request(self,path,body=None,who='student',method=None,headers=None,timeout=10):
        h={'Content-Type':'application/json',**(headers or {})}
        if who:h['Authorization']='Bearer '+self.token(who)
        req=urllib.request.Request(self.base+path,data=None if body is None else json.dumps(body,ensure_ascii=False).encode(),headers=h,method=method or ('POST' if body is not None else 'GET'))
        start=time.monotonic()
        try:
            with urllib.request.urlopen(req,timeout=timeout) as response:code=response.status;raw=response.read();response_headers=dict(response.headers)
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
    def assignment(self,title=None,subject="math"):
        code,data,_,_=self.request('/api/assignments',{'subject':subject,'title':title or 'apitest-'+self.run_id,'question':'1+1等于多少？','expected_answer':'2','max_score':10},who='teacher')
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
        return self.fixture('/control',settings)
    def fixture(self,path,body=None):
        script="const r=await fetch('http://127.0.0.1:8089'+process.argv[1],process.argv[2]?{method:'POST',headers:{'Content-Type':'application/json'},body:process.argv[2]}:{});const text=await r.text();if(!r.ok)process.exit(1);process.stdout.write(text,()=>process.exit(0))"
        return json.loads(self.compose('exec','-T','gateway','node','--input-type=module','-e',script,path,'' if body is None else json.dumps(body)))
    def cleanup(self):
        if self.assignments:
            ids=','.join(str(int(x)) for x in self.assignments)
            self.sql(f'DELETE FROM submissions WHERE assignment_id IN ({ids}); DELETE FROM grade_assignments WHERE id IN ({ids});')
        for who in list(self.tokens):
            try:self.request('/api/logout',{},who=who)
            except Exception:pass
        self.close_sql()
    def discard(self,assignment):
        assignment=int(assignment)
        if assignment not in self.assignments:raise ValueError('不能清理非本次创建的作业')
        self.sql(f'DELETE FROM submissions WHERE assignment_id={assignment}; DELETE FROM grade_assignments WHERE id={assignment};')
        self.assignments.remove(assignment)
    def queues(self):return json.loads(self.compose('exec','-T','worker','/app/grade-worker','-queues'))
