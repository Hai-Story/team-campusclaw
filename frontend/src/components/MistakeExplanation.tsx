import { useEffect, useRef, useState } from 'react'
import { homework } from '../grading'

export default function MistakeExplanation({submission,onOpenMaterial}:{submission:number;onOpenMaterial?:(id:number)=>void}){
 const current=useRef<AbortController|null>(null)
 const [citations,setCitations]=useState<Awaited<ReturnType<typeof homework.explanation>>['citations']>([])
 const [text,setText]=useState(''),[error,setError]=useState(''),[busy,setBusy]=useState(false)
 useEffect(()=>()=>{current.current?.abort()},[submission])
 async function generate(){
  const control=new AbortController();current.current?.abort();current.current=control
  setBusy(true);setError('');setText('');setCitations([])
  try{const result=await homework.explanation(submission,control.signal);if(current.current===control&&!control.signal.aborted){setText(result.explanation);setCitations(result.citations??[])}}
  catch(e){if(current.current===control&&!control.signal.aborted)setError(e instanceof Error?e.message:'讲解生成失败，请稍后重试')}
  finally{if(current.current===control){current.current=null;setBusy(false)}}
 }

 function cancel(){current.current?.abort();current.current=null;setBusy(false)}
 return <section className="mistake-explanation" aria-label="错题讲解"><div className="toolbar"><button disabled={busy} onClick={()=>void generate()}>{busy?'正在生成讲解…':error?'重试讲解':'错题讲解'}</button>{busy&&<button onClick={cancel}>取消生成</button>}</div>
  <div aria-live="polite">{busy&&<p className="hint" role="status">正在分析这道题…</p>}{error&&<p className="form-error" role="alert">{error}</p>}{text&&<div className="explanation-text"><h3>错题讲解</h3><p>{text}</p>{citations.length>0&&<section className="lesson-sources"><h4>资料出处</h4>{citations.map(c=><div key={c.index}><button onClick={()=>onOpenMaterial?.(c.material_id)}>[{c.index}] {c.material_title}</button><small>片段 {c.chunk_index+1} · 字符 {c.char_start}–{c.char_end}</small><p>{c.excerpt}</p></div>)}</section>}</div>}</div>
 </section>
}
