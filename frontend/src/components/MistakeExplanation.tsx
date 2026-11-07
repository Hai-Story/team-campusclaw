import { useEffect, useRef, useState } from 'react'
import { homework } from '../grading'

export default function MistakeExplanation({submission}:{submission:number}){
 const current=useRef<AbortController|null>(null)
 const [text,setText]=useState(''),[error,setError]=useState(''),[busy,setBusy]=useState(false)
 useEffect(()=>()=>{current.current?.abort()},[submission])
 async function generate(){
  const control=new AbortController();current.current?.abort();current.current=control
  setBusy(true);setError('');setText('')
  try{const result=await homework.explanation(submission,control.signal);if(current.current===control&&!control.signal.aborted)setText(result.explanation)}
  catch(e){if(current.current===control&&!control.signal.aborted)setError(e instanceof Error?e.message:'讲解生成失败，请稍后重试')}
  finally{if(current.current===control){current.current=null;setBusy(false)}}
 }
 function cancel(){current.current?.abort();current.current=null;setBusy(false)}
 return <section className="mistake-explanation" aria-label="错题讲解"><div className="toolbar"><button disabled={busy} onClick={()=>void generate()}>{busy?'正在生成讲解…':error?'重试讲解':'错题讲解'}</button>{busy&&<button onClick={cancel}>取消生成</button>}</div>
  <div aria-live="polite">{busy&&<p className="hint" role="status">正在分析这道题…</p>}{error&&<p className="form-error" role="alert">{error}</p>}{text&&<div className="explanation-text"><h3>错题讲解</h3><p>{text}</p></div>}</div>
 </section>
}
