// Isolated OpenAI-compatible fixture. No external requests or real student data.
import http from 'node:http'
let settings={delay_ms:20,statuses:[]},counts=new Map(),calls=0
const server=http.createServer(async(req,res)=>{
 let raw='';for await(const part of req)raw+=part
 let body;try{body=JSON.parse(raw||'{}')}catch{res.writeHead(400);res.end();return}
 res.setHeader('Content-Type','application/json')
 if(req.url==='/control'&&req.method==='POST'){settings={delay_ms:20,statuses:[],...body};counts.clear();calls=0;res.end('{"ok":true}');return}
 if(req.url==='/stats'){res.end(JSON.stringify({calls,counts:Object.fromEntries(counts)}));return}
 if(req.url==='/v1/embeddings'){
  const input=Array.isArray(body.input)?body.input:[body.input];res.end(JSON.stringify({data:input.map((_,index)=>({index,embedding:[1,0,0,0,0,0,0,0]}))}));return
 }
 if(req.url!=='/v1/chat/completions'){res.writeHead(404);res.end('{}');return}
 calls++
 const task=req.headers['x-task-id']||'teaching',n=(counts.get(task)||0)+1;counts.set(task,n)
 await new Promise(resolve=>setTimeout(resolve,settings.delay_ms))
 const status=settings.statuses[Math.min(n-1,settings.statuses.length-1)]||200
 if(status!==200){res.writeHead(status);res.end('{"error":"controlled fixture error"}');return}
 const content=task==='teaching'?'请先检查题目中的已知条件，再核对每一步推导。':JSON.stringify({feedback:`第${n}次调用：已核对本次作答，请结合题目检查计算步骤。`})
 if(body.stream){res.setHeader('Content-Type','text/event-stream');res.write('data: '+JSON.stringify({choices:[{delta:{content}}]})+'\n\n');res.end('data: [DONE]\n\n');return}
 res.end(JSON.stringify({choices:[{message:{role:'assistant',content}}],usage:{prompt_tokens:20,completion_tokens:12,total_tokens:32}}))
})
server.listen(8089,'0.0.0.0')
