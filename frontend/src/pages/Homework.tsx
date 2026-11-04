import { useEffect, useState, type FormEvent } from 'react'
import type { Me } from '../api'
import { homework, readPointer, submissionKey, watchTask, type Assignment, type GradeTask, type TaskPointer } from '../grading'

const statusText={pending:'等待批改',running:'正在批改',succeeded:'批改完成',failed:'批改失败'}
const empty={title:'',question:'',expected_answer:'',max_score:10}

export default function Homework({me}:{me:Me}){
  const storageKey=`campusclaw_task:${me.id}:${me.class_id}`
  const [pointer,setPointer]=useState<TaskPointer|null>(()=>readPointer(sessionStorage.getItem(storageKey)))
  const [items,setItems]=useState<Assignment[]>([]),[selected,setSelected]=useState<number|null>(pointer?.assignment_id??null)
  const [page,setPage]=useState(1),[refresh,setRefresh]=useState(0),[loading,setLoading]=useState(true),[listError,setListError]=useState('')
  const [draft,setDraft]=useState({...empty}),[answer,setAnswer]=useState(''),[busy,setBusy]=useState(false),[formError,setFormError]=useState('')
  const [task,setTask]=useState<GradeTask|null>(null),[taskError,setTaskError]=useState(''),[pollRevision,setPollRevision]=useState(0)
  const current=items.find(item=>item.id===selected)

  useEffect(()=>{
    const control=new AbortController();setLoading(true);setListError('')
    homework.list(page,control.signal).then(data=>{if(!control.signal.aborted){setItems(data.items);setSelected(old=>old??data.items[0]?.id??null)}})
      .catch(error=>{if(!control.signal.aborted)setListError(error instanceof Error?error.message:'作业读取失败')})
      .finally(()=>{if(!control.signal.aborted)setLoading(false)})
    return()=>control.abort()
  },[page,refresh,me.id])

  useEffect(()=>{
    if(!pointer)return
    const control=new AbortController();setTaskError('')
    watchTask(pointer.task_id,control.signal,setTask).catch(error=>{if(!control.signal.aborted)setTaskError(error instanceof Error?error.message:'批改进度读取失败')})
    return()=>control.abort()
  },[pointer?.task_id,pollRevision])

  async function publish(event:FormEvent){
    event.preventDefault();setBusy(true);setFormError('')
    try{const created=await homework.create(draft);setDraft({...empty});setSelected(created.id);setPage(1);setRefresh(value=>value+1)}
    catch(error){setFormError(error instanceof Error?error.message:'发布失败，请重试')}
    finally{setBusy(false)}
  }
  async function submit(event:FormEvent){
    event.preventDefault();if(!current)return;setBusy(true);setFormError('')
    try{
      const key=await submissionKey(me.id,current.id,answer),accepted=await homework.submit(current.id,answer.trim(),key)
      const next={task_id:accepted.task_id,assignment_id:current.id,submission_id:accepted.submission_id}
      sessionStorage.setItem(storageKey,JSON.stringify(next));setTask(null);setPointer(next);setPollRevision(value=>value+1)
    }catch(error){setFormError(error instanceof Error?error.message:'提交失败，请重试；相同答案会沿用提交标识')}
    finally{setBusy(false)}
  }

  return <main className="homework-page">
    <header><div className="eyebrow">{me.class_name} · 客观题练习</div><h1>作业与批改</h1><p className="hint">{me.role==='teacher'?'发布本班练习，设置参考答案与分值。':'提交后可离开页面，稍后回来查看批改结果。'}</p></header>
    <div className="homework-grid">
      <section className="homework-panel" aria-label="本班作业">
        <h2>本班作业</h2>
        {loading?<p role="status">正在读取作业…</p>:listError?<div role="alert"><p>{listError}</p><button onClick={()=>setRefresh(value=>value+1)}>重试读取</button></div>:items.length===0?<p className="hint">{me.role==='teacher'?'发布第一份练习后，学生即可作答。':'这里还没有作业，请等待教师发布。'}</p>:<div className="homework-list">{items.map(item=><button key={item.id} aria-pressed={selected===item.id} className={selected===item.id?'selected':''} onClick={()=>{setSelected(item.id);setAnswer('');setFormError('')}}><strong>{item.title}</strong><span>{item.max_score} 分</span></button>)}</div>}
        <div className="homework-pagination"><button disabled={page===1||loading} onClick={()=>{setPage(value=>value-1);setSelected(null)}}>上一页</button><span>第 {page} 页</span><button disabled={items.length<20||loading} onClick={()=>{setPage(value=>value+1);setSelected(null)}}>下一页</button></div>
      </section>
      <section className="homework-panel" aria-label={me.role==='teacher'?'发布作业':'作答与批改结果'}>
        {me.role==='teacher'?<form onSubmit={event=>void publish(event)} className="homework-form">
          <h2>发布作业</h2>
          <label>作业标题<input value={draft.title} maxLength={120} required onChange={event=>setDraft({...draft,title:event.target.value})}/></label>
          <label>题目<textarea value={draft.question} maxLength={4000} required rows={4} onChange={event=>setDraft({...draft,question:event.target.value})}/></label>
          <label>参考答案<input value={draft.expected_answer} maxLength={2000} required onChange={event=>setDraft({...draft,expected_answer:event.target.value})}/></label>
          <label>满分<input type="number" value={draft.max_score} min={1} max={100} required onChange={event=>setDraft({...draft,max_score:Number(event.target.value)})}/></label>
          <p className="hint">分数按答案精确匹配计算，模型提供学习反馈。</p>
          <button className="primary" disabled={busy}>{busy?'正在发布…':'发布到本班'}</button>
        </form>:current?<form onSubmit={event=>void submit(event)} className="homework-form">
          <h2>{current.title}</h2><p className="homework-question">{current.question}</p>
          <label>你的答案<textarea value={answer} maxLength={10000} required rows={5} onChange={event=>setAnswer(event.target.value)}/></label>
          <button className="primary" disabled={busy||!answer.trim()}>{busy?'正在提交…':'提交批改'}</button>
        </form>:<p className="hint">选择左侧作业后开始作答。</p>}
        {formError&&<p className="form-error" role="alert">{formError}</p>}
        {me.role==='teacher'&&current&&<details className="homework-reference"><summary>查看「{current.title}」</summary><p className="homework-question">{current.question}</p><p>参考答案：{current.expected_answer}</p></details>}
        {pointer&&<section className="homework-result" aria-label="批改进度" aria-live="polite">
          <p className="hint">对应作业：{items.find(item=>item.id===pointer.assignment_id)?.title??`作业 ${pointer.assignment_id}`}</p>
          <h2>{task?statusText[task.status]:'正在读取批改进度…'}</h2>
          {taskError?<><p role="alert">{taskError}</p><button onClick={()=>setPollRevision(value=>value+1)}>重新查询</button></>:task?.status==='succeeded'&&task.result?<><p className="homework-score">{task.result.score}<span> / {task.result.max_score} 分</span></p><p>{task.result.feedback}</p></>:task?.status==='failed'?<p>暂时无法完成批改，请联系教师核对。错误类型：{task.error}</p>:<p className="hint">作业已保存，后台正在处理。刷新页面后仍可查看进度。</p>}
        </section>}
      </section>
    </div>
  </main>
}
