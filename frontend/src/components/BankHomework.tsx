import {useState} from 'react'
import type {Subject} from '../subjects'
import {answerText,questionKinds,type PaperDraft} from '../bank-api'
import type {PublishedPaper} from '../paper-api'
import PaperBuilder from './PaperBuilder'
import PublishPaper from './PublishPaper'

export default function BankHomework({subject,onCancel,onPublished,onBusy}:{subject:Subject;onCancel:()=>void;onPublished:(p:PublishedPaper)=>void;onBusy:(value:boolean)=>void}){
 const [paper,setPaper]=useState<PaperDraft|null>(null),[editing,setEditing]=useState(true),[busy,setBusy]=useState(false)
 function activity(v:boolean){setBusy(v);onBusy(v)}
 return <section className="bank-page bank-homework">
  {editing?<PaperBuilder purpose="homework" subject={subject} initial={paper} onBusy={activity} onCancel={onCancel} onSaved={v=>{setPaper(v);setEditing(false)}}/>:paper&&<section className="homework-panel" aria-label="核对题库作业">
   <div className="section-toolbar"><h2>{paper.title}</h2><button disabled={busy} onClick={onCancel}>返回发布作业</button></div>
   <p>{paper.items.length}题 · 总分{paper.total_score}分</p>
   <ol className="paper-preview">{paper.items.map((item,i)=><li key={item.question_id}><h3>第{i+1}题 · {questionKinds[item.question.type]} · {item.score}分</h3><p className="bank-stem">{item.question.stem}</p>{item.question.options.map((o,j)=><p key={j}>{String.fromCharCode(65+j)}. {o}</p>)}<p>参考答案：{answerText(item.question)}</p><p>{item.question.explanation}</p></li>)}</ol>
   <button disabled={busy} onClick={()=>setEditing(true)}>调整选题与分值</button><PublishPaper paper={paper} asHomework onBusy={activity} onPublished={onPublished}/>
  </section>}
 </section>
}
