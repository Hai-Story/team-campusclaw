// Isolated OpenAI-compatible fixture. No external requests or real student data.
import http from 'node:http'
let settings={delay_ms:20,statuses:[]},counts=new Map(),calls=0,cancelled=0,epoch=0
const waiters=new Set()
function releaseAll(){for(const release of waiters)release();waiters.clear()}
const server=http.createServer(async(req,res)=>{
 let raw='';for await(const part of req)raw+=part
 let body;try{body=JSON.parse(raw||'{}')}catch{res.writeHead(400);res.end();return}
 res.setHeader('Content-Type','application/json')
 if(req.url==='/control'&&req.method==='POST'){epoch++;releaseAll();settings={delay_ms:20,statuses:[],...body};counts.clear();calls=0;cancelled=0;res.end('{"ok":true}');return}
 if(req.url==='/release'&&req.method==='POST'){releaseAll();res.end('{"ok":true}');return}
 if(req.url==='/stats'){res.end(JSON.stringify({calls,cancelled,counts:Object.fromEntries(counts)}));return}
 if(req.url==='/v1/embeddings'){
  const input=Array.isArray(body.input)?body.input:[body.input];res.end(JSON.stringify({data:input.map((_,index)=>({index,embedding:[1,0,0,0,0,0,0,0]}))}));return
 }
 if(req.url!=='/v1/chat/completions'){res.writeHead(404);res.end('{}');return}
 const applied={...settings},activeEpoch=epoch
 let release
 res.on('close',()=>{if(!res.writableEnded&&epoch===activeEpoch)cancelled++;release?.()})
 calls++
 const task=req.headers['x-task-id']||'teaching',n=(counts.get(task)||0)+1;counts.set(task,n)
 if(String(task).startsWith('explain-')&&applied.hold_explanation)await new Promise(resolve=>{release=()=>{waiters.delete(release);resolve()};waiters.add(release)})
 await new Promise(resolve=>setTimeout(resolve,applied.delay_ms))
 if(res.destroyed)return
 const status=applied.statuses[Math.min(n-1,applied.statuses.length-1)]||200
 if(status!==200){res.writeHead(status);res.end('{"error":"controlled fixture error"}');return}
 let input={};try{input=JSON.parse(body.messages?.find(x=>x.role==='user')?.content||'{}')}catch{}
 const content=String(task).startsWith('explain-')?JSON.stringify(applied.explanation_response??{explanation:'请回想这类题目的基本定义，检查作答中采用的概念是否一致，再试着用定义验证自己的判断。'}):task==='teaching'?'请先检查题目中的已知条件，再核对每一步推导。':JSON.stringify({correct:applied.correct??(String(input.answer).trim()===String(input.expected_answer).trim()),feedback:applied.feedbacks?.[Math.min(n-1,applied.feedbacks.length-1)]??(applied.correct===false?'本次结果与参考不一致，请核对运算和符号。':'本次结果与参考一致。')})
 if(body.stream){res.setHeader('Content-Type','text/event-stream');res.write('data: '+JSON.stringify({choices:[{delta:{content}}]})+'\n\n');res.end('data: [DONE]\n\n');return}
 res.end(JSON.stringify({choices:[{message:{role:'assistant',content}}],usage:{prompt_tokens:20,completion_tokens:12,total_tokens:32}}))
})
server.listen(8089,'0.0.0.0')
