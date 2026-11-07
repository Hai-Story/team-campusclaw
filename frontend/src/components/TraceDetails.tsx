import { useState } from 'react'
import { tutorAPI,type LLMTrace } from '../tutor-api'
import Icon from './Icon'
export default function TraceDetails({conversationID,messageID}:{conversationID:number;messageID:number}){
 const [trace,setTrace]=useState<LLMTrace|null>(null),[busy,setBusy]=useState(false),[error,setError]=useState(''),[open,setOpen]=useState(false)
 async function show(){if(open){setOpen(false);return};setOpen(true);if(trace)return;setBusy(true);try{setTrace(await tutorAPI.trace(conversationID,messageID));setError('')}catch(e){setError(e instanceof Error?e.message:'读取失败')}finally{setBusy(false)}}
 return <div className="trace-details"><button className="trace-button" onClick={()=>void show()} aria-expanded={open}><Icon name="file" size={15}/>{open?'收起':'查看'}调用详情</button>{open&&<div className="trace-content"><p className="hint">实际网关请求与原始响应，仅本人可见；不记录鉴权头。{trace?.truncated?'已达到调用记录上限。':''}</p>{busy&&<p>正在读取…</p>}{error&&<p role="alert">{error}</p>}{trace?.calls.map((c,i)=><details key={i} open={i===0}><summary>{i+1}. {c.phase==='tool-planning'?'工具规划':'流式回答'} · HTTP {c.http_status||'未连接'}</summary><h4>请求 JSON</h4>{c.request_truncated?<p>请求超出记录上限，未保存正文。</p>:<pre>{JSON.stringify(c.request,null,2)}</pre>}{c.output_text&&<><h4>网关回答正文 {c.output_truncated?'（已截断）':''}</h4><pre>{c.output_text}</pre><small>{c.finish_reason?'结束原因：'+c.finish_reason:''}{c.usage?' · 用量：'+JSON.stringify(c.usage):''}</small></>}<h4>原始响应 {c.response_truncated?'（已截断）':''}</h4><pre>{c.response||c.error||'响应为空'}</pre></details>)}</div>}</div>
}
