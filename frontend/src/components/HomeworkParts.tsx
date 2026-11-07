import { subjects, effectiveCorrect, statusText, type Subject, type GradeTask } from '../grading'

export function SubjectPicker({value,onChange,disabled=false}:{value:Subject;onChange:(value:Subject)=>void;disabled?:boolean}){
 return <div className="subject-picker" role="group" aria-label="选择学科">{subjects.map(s=><button type="button" key={s.id} aria-pressed={s.id===value} disabled={disabled} onClick={()=>onChange(s.id)}>{s.name}</button>)}</div>
}
export function Pagination({page,loading,hasMore,onChange}:{page:number;loading:boolean;hasMore:boolean;onChange:(page:number)=>void}){
 return <div className="homework-pagination"><button disabled={loading||page===1} onClick={()=>onChange(page-1)}>上一页</button><span>第 {page} 页</span><button disabled={loading||!hasMore} onClick={()=>onChange(page+1)}>下一页</button></div>
}
export function Verdict({correct}:{correct:boolean|undefined}){
 return <span className={'verdict-tag '+(correct===undefined?'undecided':correct?'correct':'incorrect')}>{correct===undefined?'待判定':correct?'正确':'错误'}</span>
}
export function ResultPanel({task}:{task:GradeTask}){
 const final=task.final_review
 return <section className="homework-result" aria-label="初判与终评" aria-live="polite">
  <div className="homework-section-heading"><h3>{final?'教师已终评':task.result?'AI已初判，待教师终评':statusText[task.status]}</h3><Verdict correct={effectiveCorrect(task)}/></div>
  {task.result&&<div className="judgment-block"><h4>AI初判 · {task.result.correct?'正确':'错误'}</h4><p>{task.result.feedback}</p><small>初判用于审查与反思，最终以教师终评为准。</small></div>}
  {final?<div className="judgment-block final-review"><h4>教师终评 · {final.correct?'正确':'错误'}</h4><p className="homework-score">{final.score}<span> / {final.max_score} 分</span></p>{final.feedback&&<p>{final.feedback}</p>}<small>终评版本 {final.version} · {new Date(final.reviewed_at).toLocaleString('zh-CN')}</small></div>:<p className="hint">{task.status==='failed'?'初判未完成，教师仍可审阅本次作答。':'作答已保存，终评分数将在教师确认后显示。'}</p>}
 </section>
}
