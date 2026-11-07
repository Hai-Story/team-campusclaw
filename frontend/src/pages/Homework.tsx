import { useEffect, useRef, useState, useCallback, type FormEvent } from 'react'
import type { Me } from '../api'
import { homework, readSubject, submissionKey, watchTask, terminal, effectiveCorrect, statusText, type Assignment, type Submission, type Subject, subjects } from '../grading'
import { SubjectPicker, Pagination, ResultPanel, Verdict } from '../components/HomeworkParts'
import ReviewDialog from '../components/ReviewDialog'
import Icon from '../components/Icon'

const empty={title:'',question:'',expected_answer:'',max_score:10}
export default function Homework({me}:{me:Me}){
 const storage=`campusclaw_subject:${me.id}:${me.class_id}`
 const attempt=useRef(crypto.randomUUID())
 const [subject,setSubject]=useState<Subject>(()=>readSubject(sessionStorage.getItem(storage)))
 const [items,setItems]=useState<Assignment[]>([]),[selected,setSelected]=useState<number|null>(null)
 const [page,setPage]=useState(1),[refresh,setRefresh]=useState(0),[loading,setLoading]=useState(true),[listError,setListError]=useState('')
 const [mode,setMode]=useState<'detail'|'publish'|'grade'>('detail'),[draft,setDraft]=useState({...empty}),[answer,setAnswer]=useState('')
 const [draftSubject,setDraftSubject]=useState<Subject|''>('')
 const [busy,setBusy]=useState(false),[formError,setFormError]=useState('')
 const [mine,setMine]=useState<Submission|null>(null),[mineLoading,setMineLoading]=useState(false),[mineError,setMineError]=useState(''),[mineRefresh,setMineRefresh]=useState(0)
 const [submissions,setSubmissions]=useState<Submission[]>([]),[subPage,setSubPage]=useState(1),[subRefresh,setSubRefresh]=useState(0),[subLoading,setSubLoading]=useState(false),[subError,setSubError]=useState('')
 const [reviewing,setReviewing]=useState<Submission|null>(null)
 const closeReview=useCallback(()=>setReviewing(null),[])
 const current=items.find(item=>item.id===selected)
 function changeSubject(value:Subject){setSubject(value);setPage(1);setSelected(null);setItems([]);setMine(null);setSubmissions([]);setSubPage(1);setMode('detail');setDraft({...empty});setDraftSubject(value);setAnswer('');setFormError('')}
 useEffect(()=>{sessionStorage.setItem(storage,subject)},[storage,subject])
 useEffect(()=>{
  const control=new AbortController();setLoading(true);setListError('')
  homework.list(page,subject,control.signal).then(data=>{if(control.signal.aborted)return;setItems(data.items);const saved=Number(sessionStorage.getItem(`${storage}:${subject}`));setSelected(old=>data.items.some(x=>x.id===old)?old:data.items.some(x=>x.id===saved)?saved:data.items[0]?.id??null)})
   .catch(e=>{if(!control.signal.aborted)setListError(e instanceof Error?e.message:'作业读取失败')}).finally(()=>{if(!control.signal.aborted)setLoading(false)})
  return()=>control.abort()
 },[subject,page,refresh,storage])
 useEffect(()=>{if(current)sessionStorage.setItem(`${storage}:${subject}`,String(current.id))},[current,storage,subject])
 useEffect(()=>{
  if(me.role!=='student'||!current)return
  const control=new AbortController();setMine(null);setMineLoading(true);setMineError('')
  homework.mine(current.id,control.signal).then(value=>{if(!control.signal.aborted){setMine(value);setAnswer(value?.answer??'')}}).catch(e=>{if(!control.signal.aborted)setMineError(e instanceof Error?e.message:'作答读取失败')}).finally(()=>{if(!control.signal.aborted)setMineLoading(false)})
  return()=>control.abort()
 },[current?.id,mineRefresh,me.role])
 useEffect(()=>{
  if(!mine||terminal(mine.task))return
  const control=new AbortController(),id=mine.task.task_id
  watchTask(id,control.signal,task=>setMine(old=>old?.task.task_id===id?{...old,task}:old)).catch(e=>{if(!control.signal.aborted)setMineError(e instanceof Error?e.message:'进度读取失败')})
  return()=>control.abort()
 },[mine?.task.task_id,mineRefresh])
 useEffect(()=>{
  if(me.role!=='teacher'||mode!=='grade'||!current)return
  const control=new AbortController();setSubLoading(true);setSubError('')
  homework.submissions(current.id,subPage,control.signal).then(value=>{if(!control.signal.aborted)setSubmissions(value.items)}).catch(e=>{if(!control.signal.aborted)setSubError(e instanceof Error?e.message:'提交读取失败')}).finally(()=>{if(!control.signal.aborted)setSubLoading(false)})
  return()=>control.abort()
 },[current?.id,mode,subPage,subRefresh,me.role])
 async function publish(event:FormEvent){
  event.preventDefault();if(!draftSubject){setFormError('请选择作业学科');return};setBusy(true);setFormError('')
  try{const value=await homework.create({...draft,subject:draftSubject});setSubject(draftSubject);setSelected(value.id);setDraft({...empty});setPage(1);setRefresh(x=>x+1);setMode('detail')}
  catch(e){setFormError(e instanceof Error?e.message:'发布失败，请重试')}finally{setBusy(false)}
 }
 async function submit(event:FormEvent){
  event.preventDefault();if(!current)return;setBusy(true);setFormError('')
  try{const key=await submissionKey(me.id,current.id,answer,attempt.current);await homework.submit(current.id,answer.trim(),key);attempt.current=crypto.randomUUID();setMineRefresh(x=>x+1)}
  catch(e){setFormError(e instanceof Error?e.message:'提交失败，请重试')}finally{setBusy(false)}
 }
 return <main className="homework-page">
  <header className="page-heading"><div><div className="eyebrow">{me.class_name} · 作业练习</div><h1>{me.role==='teacher'?'作业发布与批改':'提交作业'}</h1><p className="hint">{me.role==='teacher'?'按学科发布本班作业、审阅学生作答与AI初判，并确认终评分数和对错。':'按学科完成练习，查看AI初判与教师终评，结合反馈反思作答。'}</p></div>
   {me.role==='teacher'&&<div className="heading-actions"><button disabled={busy} onClick={()=>{setMode('publish');setDraftSubject(subject);setFormError('')}}><Icon name="plus" size={18}/>发布作业</button><button className={mode==='grade'?'primary':''} disabled={!current||busy} onClick={()=>{setMode('grade');setSubPage(1)}}><Icon name="clipboard" size={18}/>批改作业</button></div>}
  </header><SubjectPicker value={subject} onChange={changeSubject} disabled={busy}/>
  <div className="homework-grid"><section className="homework-panel" aria-label="本班作业"><h2>本班作业</h2>
   {loading?<p role="status">正在读取作业…</p>:listError?<div role="alert"><p>{listError}</p><button onClick={()=>setRefresh(x=>x+1)}>重试读取</button></div>:items.length===0?<p className="hint">{me.role==='teacher'?'这个学科还没有作业，点击发布作业开始。':'这个学科还没有作业，请等待教师发布。'}</p>:<div className="homework-list">{items.map(item=><button key={item.id} disabled={busy} aria-pressed={selected===item.id} className={selected===item.id?'selected':''} onClick={()=>{setSelected(item.id);setMode('detail');setFormError('');setSubPage(1);setMine(null);setMineRefresh(x=>x+1);setAnswer('')}}><strong>{item.title}</strong><span>{subjects.find(s=>s.id===item.subject)?.name} · 满分 {item.max_score}</span></button>)}</div>}
   <Pagination page={page} loading={loading||busy} hasMore={items.length>=20} onChange={value=>{setPage(value);setSelected(null);setMine(null);setMode('detail')}}/>
  </section><section className="homework-panel" aria-label={mode==='publish'?'发布作业':mode==='grade'?'批改作业':'作业内容'}>
   {me.role==='teacher'&&mode==='publish'?<form className="homework-form" onSubmit={event=>void publish(event)}><div className="homework-section-heading"><h2>发布作业</h2><button type="button" disabled={busy} onClick={()=>setMode('detail')}>返回详情</button></div>
    <label>学科<select required value={draftSubject} onChange={e=>setDraftSubject(e.target.value as Subject)}><option value="">请选择学科</option>{subjects.map(s=><option key={s.id} value={s.id}>{s.name}</option>)}</select></label><label>作业标题<input required maxLength={120} value={draft.title} onChange={e=>setDraft({...draft,title:e.target.value})}/></label><label>题目<textarea required rows={4} maxLength={4000} value={draft.question} onChange={e=>setDraft({...draft,question:e.target.value})}/></label><label>参考答案<textarea required rows={3} maxLength={2000} value={draft.expected_answer} onChange={e=>setDraft({...draft,expected_answer:e.target.value})}/></label><label>满分<input type="number" required min={1} max={100} step={1} value={draft.max_score} onChange={e=>setDraft({...draft,max_score:Number(e.target.value)})}/></label><button className="primary" disabled={busy}>{busy?'正在发布…':'发布到本班'}</button>
   </form>:current?<>
    <div className="homework-section-heading"><h2>{current.title}</h2>{me.role==='teacher'&&mode==='grade'&&<button onClick={()=>setMode('detail')}>返回详情</button>}</div><p className="homework-question">{current.question}</p><p className="hint">{subjects.find(s=>s.id===current.subject)?.name} · 满分 {current.max_score} 分</p>
    {me.role==='teacher'?mode==='grade'?<section aria-label="学生提交"><div className="homework-section-heading"><h3>学生提交</h3><button onClick={()=>setSubRefresh(x=>x+1)} disabled={subLoading}>刷新提交</button></div>
     {subLoading?<p role="status">正在读取学生提交…</p>:subError?<div role="alert"><p>{subError}</p><button onClick={()=>setSubRefresh(x=>x+1)}>重试</button></div>:submissions.length===0?<p className="hint">这份作业还没有学生提交。</p>:<div className="submission-list">{submissions.map(row=><article key={row.id} className="submission-card"><div className="homework-section-heading"><strong>{row.student_name}</strong><span className="hint">{row.latest?'最新作答':'历史作答'}</span></div><p className="student-answer">{row.answer}</p><div className="submission-status"><span>AI初判：<Verdict correct={row.task.result?.correct}/></span><span>教师终评：{row.task.final_review?<>{row.task.final_review.score} / {row.max_score} 分 · <Verdict correct={row.task.final_review.correct}/></>:'待确认'}</span></div><p className="hint">{new Date(row.submitted_at).toLocaleString('zh-CN')} · {row.task.final_review?'已终评':statusText[row.task.status]}</p><button className="primary" onClick={()=>setReviewing(row)}>{row.task.final_review?'修改终评':'审阅与终评'}</button></article>)}</div>}
     <Pagination page={subPage} loading={subLoading} hasMore={submissions.length>=20} onChange={setSubPage}/>
    </section>:<div className="homework-reference"><h3>参考答案</h3><p className="homework-question">{current.expected_answer}</p><p className="hint">点击“批改作业”查看学生提交并确认终评。</p></div>:<>
     {mineLoading?<p role="status">正在读取我的作答…</p>:mineError?<div role="alert"><p>{mineError}</p><button onClick={()=>setMineRefresh(x=>x+1)}>重新读取</button></div>:<><form className="homework-form" onSubmit={event=>void submit(event)}><label>我的作答<textarea required rows={5} maxLength={10000} value={answer} onChange={e=>setAnswer(e.target.value)}/></label><div className="heading-actions"><button className="primary" disabled={busy||!answer.trim()}>{busy?'正在提交…':mine?'重新提交作答':'提交作业'}</button>{mine&&<button type="button" onClick={()=>setMineRefresh(x=>x+1)} disabled={busy}>刷新结果</button>}</div></form>{mine&&<><p className="hint">当前有效判断：<Verdict correct={effectiveCorrect(mine.task)}/></p><ResultPanel task={mine.task}/></>}</>}
    </>}
   </>:<p className="hint">选择本学科的作业后查看内容。</p>}{formError&&<p className="form-error" role="alert">{formError}</p>}
  </section></div>
  {reviewing&&<ReviewDialog key={reviewing.id} submission={reviewing} reference={current?.expected_answer} onClose={closeReview} onSaved={()=>{setReviewing(null);setSubRefresh(x=>x+1)}}/>}
 </main>
}
