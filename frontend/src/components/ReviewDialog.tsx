import { useEffect, useRef, useState, type FormEvent } from 'react'
import { homework, type Submission } from '../grading'
import { ResultPanel } from './HomeworkParts'
import Icon from './Icon'

export default function ReviewDialog({submission,reference,onClose,onSaved}:{submission:Submission;reference?:string;onClose:()=>void;onSaved:()=>void}){
 const panel=useRef<HTMLElement>(null)
 const [score,setScore]=useState(submission.task.final_review?.score.toString()??'')
 const [correct,setCorrect]=useState<boolean|null>(submission.task.final_review?.correct??submission.task.result?.correct??null)
 const [feedback,setFeedback]=useState(submission.task.final_review?.feedback??'')
 const [saving,setSaving]=useState(false),[error,setError]=useState('')
 useEffect(()=>{
  const previous=document.activeElement as HTMLElement|null
  panel.current?.querySelector<HTMLElement>('button,input')?.focus()
  function key(event:KeyboardEvent){
   if(event.key==='Escape'&&!saving){event.preventDefault();onClose()}
   if(event.key==='Tab'){
    const nodes=[...panel.current?.querySelectorAll<HTMLElement>('button:not([disabled]),input:not([disabled]),textarea:not([disabled])')??[]]
    const first=nodes[0],last=nodes[nodes.length-1]
    if(event.shiftKey&&document.activeElement===first){event.preventDefault();last?.focus()}
    else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first?.focus()}
   }
  }
  document.addEventListener('keydown',key);return()=>{document.removeEventListener('keydown',key);previous?.focus()}
 },[onClose,saving])
 async function save(event:FormEvent){
  event.preventDefault();if(correct===null||score==='')return
  setSaving(true);setError('')
  try{await homework.review(submission.id,{score:Number(score),correct,feedback,expected_version:submission.task.review_version});onSaved()}
  catch(e){setError(e instanceof Error?e.message:'终评保存失败，请重试')}
  finally{setSaving(false)}
 }
 return <div className="modal-mask" onMouseDown={event=>{if(event.target===event.currentTarget&&!saving)onClose()}}><section ref={panel} className="review-dialog" role="dialog" aria-modal="true" aria-labelledby="review-title">
  <header className="editor-heading"><div><div className="eyebrow">{submission.student_name} · {submission.latest?'最新作答':'历史作答'}</div><h2 id="review-title">教师终评</h2></div><button className="icon-button" aria-label="关闭终评" disabled={saving} onClick={onClose}><Icon name="close"/></button></header>
  <form onSubmit={event=>void save(event)}><div className="review-content"><h3>{submission.title}</h3><p className="homework-question">{submission.question}</p>{reference&&<details><summary>教师参考答案</summary><p className="homework-question">{reference}</p></details>}<h4>学生作答</h4><p className="student-answer">{submission.answer}</p><ResultPanel task={submission.task}/>
   <div className="homework-form"><fieldset disabled={saving}><legend>最终判定</legend><div className="review-options">
    <label className="review-choice correct"><input type="radio" name="correct" required checked={correct===true} onChange={()=>setCorrect(true)}/><span>正确</span></label>
    <label className="review-choice incorrect"><input type="radio" name="correct" required checked={correct===false} onChange={()=>setCorrect(false)}/><span>错误</span></label>
   </div></fieldset>
   <label>终评分数（满分 {submission.max_score}）<input type="number" required min={0} max={submission.max_score} step={1} value={score} disabled={saving} onChange={event=>setScore(event.target.value)}/></label>
   <label>终评评语（选填）<textarea rows={3} maxLength={2000} value={feedback} disabled={saving} onChange={event=>setFeedback(event.target.value)}/></label></div>
   {error&&<div className="form-error" role="alert"><p>{error}</p><p>若终评已更新，请关闭后刷新提交列表，再核对并保存。</p></div>}
  </div><footer className="editor-footer"><button type="button" disabled={saving} onClick={onClose}>取消</button><button className="primary" disabled={saving||correct===null||score===''}>{saving?'正在保存…':'确认终评'}</button></footer></form>
 </section></div>
}
