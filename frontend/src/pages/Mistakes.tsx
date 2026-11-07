import { useEffect, useState } from 'react'
import type { Me } from '../api'
import { homework, readSubject, type Subject, type Submission, subjects } from '../grading'
import { SubjectPicker, Pagination, ResultPanel } from '../components/HomeworkParts'
import MistakeExplanation from '../components/MistakeExplanation'

export default function Mistakes({me}:{me:Me}){
 const key=`campusclaw_subject:${me.id}:${me.class_id}`
 const [subject,setSubject]=useState<Subject>(()=>readSubject(sessionStorage.getItem(key)))
 const [items,setItems]=useState<Submission[]>([]),[page,setPage]=useState(1),[loading,setLoading]=useState(true),[error,setError]=useState(''),[refresh,setRefresh]=useState(0)
 useEffect(()=>{
  sessionStorage.setItem(key,subject);const control=new AbortController();setLoading(true);setError('')
  homework.mistakes(subject,page,control.signal).then(value=>{if(!control.signal.aborted)setItems(value.items)}).catch(e=>{if(!control.signal.aborted)setError(e instanceof Error?e.message:'错题读取失败')}).finally(()=>{if(!control.signal.aborted)setLoading(false)})
  return()=>control.abort()
 },[subject,page,refresh,key])
 return <main className="homework-page"><header className="page-heading"><div><div className="eyebrow">{me.class_name} · 作业练习</div><h1>我的错题</h1><p className="hint">按学科回看最新作答中的错题与反馈。教师终评优先，改判正确后会从这里移除。</p></div><button disabled={loading} onClick={()=>setRefresh(x=>x+1)}>刷新错题</button></header>
 <SubjectPicker value={subject} onChange={value=>{setSubject(value);setPage(1);setItems([])}}/>
 {loading?<p role="status">正在读取错题…</p>:error?<div className="homework-panel" role="alert"><p>{error}</p><button onClick={()=>setRefresh(x=>x+1)}>重试读取</button></div>:items.length===0?<div className="homework-panel empty-mistakes"><h2>这个学科还没有错题</h2><p className="hint">完成作业后，被判错的最新作答会显示在这里。</p></div>:<div className="mistake-list">{items.map(item=><article className="homework-panel" key={item.id}><span className="verdict-tag incorrect">{item.task.final_review?'教师终评判错':'AI初判判错 · 待教师终评'}</span><h2>{item.title}</h2><p className="hint">{subjects.find(s=>s.id===item.subject)?.name}</p><h3>题目</h3><p className="homework-question">{item.question}</p><h3>我的作答</h3><p className="student-answer">{item.answer}</p><ResultPanel task={item.task}/><MistakeExplanation submission={item.id}/></article>)}</div>}
 <Pagination page={page} loading={loading} hasMore={items.length>=20} onChange={setPage}/>
 </main>
}
