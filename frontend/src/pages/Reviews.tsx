import {useEffect,useRef,useState,type FormEvent} from 'react'
import type {Me} from '../api'
import {readSubject} from '../grading'
import type {Subject} from '../subjects'
import {reviewAPI,type ReviewSettings,type ReviewCard,type ReviewReport,type ReviewCompletion} from '../review-api'
import {knowledgeAPI,type KnowledgePoint} from '../knowledge-api'
import {SubjectPicker,Pagination} from '../components/HomeworkParts'
const text=(e:unknown)=>e instanceof Error?e.message:'操作失败，请重试'
const date=(s:string)=>new Date(s).toLocaleString('zh-CN',{timeZone:'Asia/Shanghai',hour12:false})
function Settings({initial,onChanged}:{initial:ReviewSettings;onChanged:()=>void}){
 const [settings,setSettings]=useState(initial),[values,setValues]=useState(initial.intervals.map(String)),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('')
 const action=useRef<AbortController|null>(null)
 useEffect(()=>()=>action.current?.abort(),[])
 async function run(save:boolean){const c=new AbortController();action.current=c;setBusy(true);setError('');setNotice('')
  try{const v=save?await reviewAPI.saveSettings(values.map(Number),settings.version,c.signal):await reviewAPI.settings(c.signal);if(c.signal.aborted)return;setSettings(v);setValues(v.intervals.map(String));setNotice(save?'间隔已保存，将用于之后的复习反馈。':'已读取当前间隔。');if(save)onChanged()}
  catch(e){if(!c.signal.aborted)setError(text(e))}finally{if(!c.signal.aborted)setBusy(false)}
 }
 function save(e:FormEvent){e.preventDefault();const n=values.map(Number);if(n.some((v,i)=>!Number.isInteger(v)||v<1||v>365||i>0&&v<=n[i-1])){setError('请填写五个从小到大的整数，每项为1至365天。');return};void run(true)}
 return <details className="homework-panel review-settings"><summary>复习间隔设置</summary><p className="hint">按熟悉等级安排下一次复习；调整只用于之后的反馈，已有到期时间保留。</p><form onSubmit={save}><div className="review-intervals">{values.map((v,i)=><label key={i}>第{i+1}级（天）<input aria-label={`第${i+1}级间隔`} type="number" required min={1} max={365} step={1} value={v} disabled={busy} onChange={e=>setValues(old=>old.map((x,j)=>i===j?e.target.value:x))}/></label>)}</div><div className="toolbar"><button className="primary" disabled={busy}>{busy?'正在保存…':'保存间隔'}</button><button type="button" disabled={busy} onClick={()=>void run(false)}>重新读取设置</button></div></form>{error&&<p role="alert" className="form-error">{error} 可重新读取设置后核对。</p>}{notice&&<p role="status">{notice}</p>}</details>
}
export default function Reviews({me,onChanged,onOpenAssignment,onOpenMaterial}:{me:Me;onChanged:()=>void;onOpenAssignment:(id:number,subject:Subject)=>void;onOpenMaterial:(id:number)=>void}){
 const key=`campusclaw_review_subject:${me.id}:${me.class_id}`
 const [subject,setSubject]=useState<Subject>(()=>readSubject(sessionStorage.getItem(key))),[view,setView]=useState<'due'|'all'|'history'>('due'),[page,setPage]=useState(1),[refresh,setRefresh]=useState(0)
 const [report,setReport]=useState<ReviewReport|null>(null),[history,setHistory]=useState<ReviewCompletion[]>([]),[settings,setSettings]=useState<ReviewSettings|null>(null),[loading,setLoading]=useState(true),[error,setError]=useState('')
 const [busy,setBusy]=useState<number|null>(null),[actionError,setActionError]=useState(''),[notice,setNotice]=useState(''),[point,setPoint]=useState<KnowledgePoint|null>(null),[pointError,setPointError]=useState(''),[pointBusy,setPointBusy]=useState(false)
 const operation=useRef<AbortController|null>(null),pointRequest=useRef<AbortController|null>(null),requests=useRef(new Map<string,string>())
 useEffect(()=>{const c=new AbortController();sessionStorage.setItem(key,subject);setLoading(true);setError('');setReport(null);setPoint(null);setPointError('');pointRequest.current?.abort();setPointBusy(false)
  reviewAPI.sync(c.signal).then(()=>Promise.all([reviewAPI.list(subject,view==='all'?'all':'due',view==='history'?1:page,c.signal),view==='history'?reviewAPI.history(subject,page,c.signal):Promise.resolve(null)])).then(([r,h])=>{if(c.signal.aborted)return;setReport(r);setHistory(h?.items??[]);setSettings(old=>old??r.settings);onChanged()}).catch(e=>{if(!c.signal.aborted)setError(text(e))}).finally(()=>{if(!c.signal.aborted)setLoading(false)})
  return()=>{c.abort();pointRequest.current?.abort()}
 },[subject,view,page,refresh,key,onChanged])
 useEffect(()=>{const timer=setInterval(()=>{if(!operation.current)setRefresh(v=>v+1)},60000);return()=>{clearInterval(timer);operation.current?.abort();pointRequest.current?.abort()}},[])
 async function complete(card:ReviewCard,feedback:'good'|'again'){
  const c=new AbortController();operation.current=c;setBusy(card.id);setActionError('');setNotice('');const tag=`${card.id}:${card.version}:${feedback}`
  let requestKey=requests.current.get(tag);if(!requestKey){requestKey=crypto.randomUUID();requests.current.set(tag,requestKey)}
  try{const v=await reviewAPI.complete(card,feedback,requestKey,c.signal);if(c.signal.aborted)return;requests.current.delete(tag);setNotice(`已记录，下次复习：${date(v.due_at)}`);setRefresh(x=>x+1);onChanged()}
  catch(e){if(!c.signal.aborted)setActionError(text(e))}finally{if(operation.current===c){operation.current=null;if(!c.signal.aborted)setBusy(null)}}
 }
 async function openPoint(id:number){const c=new AbortController();pointRequest.current?.abort();pointRequest.current=c;setPointBusy(true);setPointError('');setPoint(null);try{const p=await knowledgeAPI.get(id,c.signal);if(!c.signal.aborted)setPoint(p)}catch(e){if(!c.signal.aborted)setPointError(text(e))}finally{if(!c.signal.aborted)setPointBusy(false)}}
 const changed=()=>{setPage(1);setNotice('');setActionError('')}
 return <main className="homework-page review-page"><header className="page-heading"><div><div className="eyebrow">{me.class_name} · 巩固与回顾</div><h1>间隔复习</h1><p className="hint">从错题和收藏中安排复习，用学习自评记录熟悉程度。</p></div><button disabled={loading||busy!==null} onClick={()=>setRefresh(v=>v+1)}>刷新复习</button></header>
 <SubjectPicker value={subject} disabled={busy!==null} onChange={v=>{if(v!==subject){setSubject(v);changed()}}}/>
 <div className="review-tabs" role="group" aria-label="复习视图">{([['due','到期任务'],['all','全部卡片'],['history','完成记录']] as const).map(([id,label])=><button key={id} aria-pressed={view===id} disabled={busy!==null} onClick={()=>{if(id!==view){setView(id);changed()}}}>{label}</button>)}</div>
 {notice&&<p role="status" className="review-notice">{notice}</p>}{actionError&&<div role="alert" className="form-error"><p>{actionError}</p><button onClick={()=>setRefresh(v=>v+1)}>刷新列表</button></div>}
 {loading?<p role="status">正在整理复习任务…</p>:error?<section className="homework-panel" role="alert"><p>{error}</p><button onClick={()=>setRefresh(v=>v+1)}>重试读取</button></section>:report&&<>
 <p className="hint">本学科有{report.due_count}张卡片待复习 · 到期时间使用中国标准时间</p>
 {view==='history'?<>{history.length===0?<p className="empty-state">完成复习后，记录会保存在这里。</p>:history.map(h=><article className="homework-panel" key={h.id}><div className="homework-section-heading"><h2>{h.card.title}</h2><span>{h.feedback==='good'?'已掌握':'继续巩固'}</span></div><p className="hint">{date(h.completed_at)} · 熟悉等级 {h.previous_level} → {h.level}</p><p>下次复习：{date(h.due_at)}</p><details><summary>查看复习内容</summary><p className="review-card-body">{h.card.body}</p></details></article>)}<Pagination page={page} loading={loading} hasMore={history.length===20} onChange={setPage}/></>:<>
 {report.items.length===0?<p className="empty-state">{view==='due'?'当前没有到期任务，可以查看全部卡片或收藏知识点。':'错题和收藏知识点会整理成复习卡。'}</p>:report.items.map(card=>{const due=new Date(card.due_at).getTime()<=new Date(report.server_time).getTime();return <article className="homework-panel review-card" key={card.id}><div className="homework-section-heading"><div><span className="hint">{card.source_kind==='mistake'?'错题巩固':'收藏知识点'}</span><h2>{card.title}</h2></div><span className="review-level">熟悉等级 {card.level}/5</span></div><p className="review-card-body">{card.body}</p><p className="hint">{due?'已到期':'下次复习'}：{date(card.due_at)}</p><div className="toolbar">{card.source_kind==='favorite'?<button disabled={busy!==null} onClick={()=>void openPoint(card.source_id)}>回看知识点</button>:card.assignment_id&&<button disabled={busy!==null} onClick={()=>onOpenAssignment(card.assignment_id!,card.subject)}>查看作业</button>}<button disabled={busy!==null||!due} onClick={()=>void complete(card,'again')}>还需巩固</button><button className="primary" disabled={busy!==null||!due} onClick={()=>void complete(card,'good')}>{busy===card.id?'正在记录…':'已掌握'}</button></div></article>})}
 <Pagination page={page} loading={loading||busy!==null} hasMore={page*report.page_size<report.total} onChange={setPage}/></>}
 </>}
 {pointBusy&&<p role="status">正在读取知识点…</p>}{pointError&&<p role="alert" className="form-error">{pointError}</p>}{point&&<section className="homework-panel" aria-label="知识点内容"><div className="homework-section-heading"><h2>{point.name}</h2><button onClick={()=>setPoint(null)}>收起知识点</button></div><p className="hint">{point.chapter}</p><p className="review-card-body">{point.description}</p><div className="toolbar">{point.material_ids.map((id,i)=><button key={id} onClick={()=>onOpenMaterial(id)}>关联资料{i+1}</button>)}{point.assignment_ids.map((id,i)=><button key={id} onClick={()=>onOpenAssignment(id,point.subject)}>关联练习{i+1}</button>)}</div></section>}
 {settings&&<Settings initial={settings} onChanged={()=>{setRefresh(v=>v+1);onChanged()}}/>}
 </main>
}
