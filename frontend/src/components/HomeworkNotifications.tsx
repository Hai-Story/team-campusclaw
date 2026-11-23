import {useEffect,useRef,useState} from 'react'
import {noticesAPI,watchNotices,type HomeworkNotice,type NoticePage} from '../homework-notifications'
import {subjects,type Subject} from '../subjects'
import Icon from './Icon'
export type HomeworkNoticeSummary={count:number|null;error:boolean}
export default function HomeworkNotifications({visible,onSummary,onAssignment,onPaper}:{visible:boolean;onSummary:(value:HomeworkNoticeSummary)=>void;onAssignment:(id:number,subject:Subject)=>void;onPaper:(id:number,subject:Subject)=>void}){
 const [open,setOpen]=useState(false),[page,setPage]=useState(1),[refresh,setRefresh]=useState(0),[data,setData]=useState<NoticePage|null>(null),[error,setError]=useState(false),[busy,setBusy]=useState(false),[actionError,setActionError]=useState('')
 const button=useRef<HTMLButtonElement>(null),action=useRef<AbortController|null>(null)
 useEffect(()=>()=>action.current?.abort(),[])
 useEffect(()=>{if(busy)return;return watchNotices(signal=>noticesAPI.list(page,signal),value=>{setData(value);setError(false)},()=>setError(true))},[page,refresh,busy])
 useEffect(()=>{onSummary({count:data?.unread_count??null,error})},[data?.unread_count,error,onSummary])
 useEffect(()=>{if(!visible){setOpen(false);action.current?.abort();setBusy(false);setActionError('')}},[visible])
 function close(){setOpen(false);button.current?.focus()}
 async function read(n:HomeworkNotice,go:boolean){
  if(busy)return;const c=new AbortController();action.current=c;setBusy(true);setActionError('')
  try{await noticesAPI.read(n.id,c.signal);if(c.signal.aborted)return;setData(v=>v?{...v,unread_count:Math.max(0,v.unread_count-(n.read_at?0:1)),items:v.items.map(x=>x.id===n.id?{...x,read_at:x.read_at||new Date().toISOString()}:x)}:v);if(go){close();(n.kind==='paper'?onPaper:onAssignment)(n.resource_id,n.subject)}}catch(e){if(!c.signal.aborted)setActionError(e instanceof Error?e.message:'未能保存已读，请重试')}finally{if(!c.signal.aborted){setBusy(false);setRefresh(v=>v+1)}}
 }
 if(!visible)return null
 return <section className="homework-notices" aria-label="作业提醒" onKeyDown={e=>{if(e.key==='Escape'&&open){e.preventDefault();close()}}}>
  <div className="notice-heading"><button ref={button} aria-expanded={open} aria-controls="homework-notice-panel" onClick={()=>setOpen(v=>!v)}><Icon name="bell"/>作业提醒{data&&data.unread_count>0&&<span className="review-badge" aria-label={`${data.unread_count}份新作业未读`}>{data.unread_count}</span>}{error&&<span aria-label="提醒读取失败">!</span>}</button></div>
  {open&&<div id="homework-notice-panel" className="homework-panel notice-panel"><div className="section-toolbar"><h2>作业提醒</h2><div className="toolbar"><button disabled={busy} onClick={()=>setRefresh(v=>v+1)}>刷新提醒</button><button onClick={close}>关闭提醒</button></div></div>
   {error&&<p role="alert">提醒读取失败，请刷新重试；上次读取的提醒仍保留。</p>}{actionError&&<p role="alert">{actionError}</p>}
   {!data&&!error?<p role="status">正在读取提醒…</p>:data&&<><p className="hint" role="status">{data.unread_count}份未读</p>{data.page!==page?<p role="status">正在读取此页…</p>:data.items.length===0?<p className="hint">还没有作业提醒。</p>:<ul className="notice-list">{data.items.map(n=><li key={n.id} className={n.read_at?'':'unread'}><div><strong>{n.title}</strong><p className="hint">{subjects.find(s=>s.id===n.subject)?.name} · {n.kind==='paper'?'多题作业':'单题作业'} · {new Date(n.created_at).toLocaleString('zh-CN')}{n.read_at?' · 已读':' · 未读'}</p></div><div className="toolbar"><button disabled={busy} onClick={()=>void read(n,true)}>查看作业</button>{!n.read_at&&<button disabled={busy} onClick={()=>void read(n,false)}>标为已读</button>}</div></li>)}</ul>}<div className="toolbar"><button disabled={busy||page===1} onClick={()=>setPage(v=>v-1)}>上一页提醒</button><span>第{page}页</span><button disabled={busy||data.page!==page||data.items.length<20} onClick={()=>setPage(v=>v+1)}>下一页提醒</button></div></>}
  </div>}
 </section>
}
