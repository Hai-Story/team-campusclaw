import { request } from './api.ts'

import { subjects, type Subject } from './subjects.ts'
export { subjects } from './subjects.ts'
export type { Subject } from './subjects.ts'
export function readSubject(raw:string|null):Subject{return raw==='physics'||raw==='chemistry'?'science':subjects.some(x=>x.id===raw)?raw as Subject:'math'}
export interface Assignment { id:number; subject:Subject; title:string; question:string; expected_answer?:string; max_score:number; created_at?:string }
export interface FinalReview {score:number;max_score:number;correct:boolean;feedback:string;version:number;reviewed_at:string}
export interface GradeTask {
  task_id:string; submission_id:number; status:'pending'|'running'|'succeeded'|'failed'; attempts:number; error?:string;
  result?:{correct:boolean;feedback:string}; final_review?:FinalReview;review_version:number
}
export interface Submission {id:number;assignment_id:number;student_id:number;student_name:string;answer:string;submitted_at:string;subject:Subject;title:string;question:string;max_score:number;latest:boolean;task:GradeTask}
export interface TaskPointer { task_id:string; assignment_id:number; submission_id:number }
export interface Page<T>{items:T[];page:number;page_size:number}
export const terminal = (task:GradeTask) => task.status==='succeeded'||task.status==='failed'
export const effectiveCorrect = (task:GradeTask):boolean|undefined => task.final_review?.correct ?? task.result?.correct
export const statusText={pending:'等待初判',running:'正在初判',succeeded:'待教师终评',failed:'初判未完成'}
export const homework = {
  list:(page:number,subject:Subject,signal?:AbortSignal)=>request<Page<Assignment>>(`/api/assignments?page=${page}&subject=${subject}`,{signal}),
  create:(input:Omit<Assignment,'id'|'created_at'>)=>request<Assignment>('/api/assignments',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(input)}),
  submit:(id:number,answer:string,key:string)=>request<Pick<GradeTask,'task_id'|'submission_id'|'status'>&{status_url:string}>(`/api/assignments/${id}/submissions`,{method:'POST',headers:{'Content-Type':'application/json','Idempotency-Key':key},body:JSON.stringify({answer})}),
  task:(id:string,signal:AbortSignal)=>request<GradeTask>('/api/tasks/'+id,{signal}),
  mine:(id:number,signal?:AbortSignal)=>request<Submission|null>(`/api/assignments/${id}/my-submission`,{signal}),
  submissions:(id:number,page:number,signal?:AbortSignal)=>request<Page<Submission>>(`/api/assignments/${id}/submissions?page=${page}`,{signal}),
  mistakes:(subject:Subject,page:number,signal?:AbortSignal)=>request<Page<Submission>>(`/api/mistakes?subject=${subject}&page=${page}`,{signal}),
  explanation:(id:number,signal:AbortSignal)=>request<{explanation:string}>(`/api/submissions/${id}/explanation`,{method:'POST',headers:{'Content-Type':'application/json'},body:'{}',signal}),
  review:(id:number,input:{score:number;correct:boolean;feedback:string;expected_version:number})=>request<FinalReview>(`/api/submissions/${id}/review`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(input)}),
}

export async function submissionKey(user:number,assignment:number,answer:string,attempt:string):Promise<string>{
  const digest=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(`${user}:${assignment}:${attempt}:${answer.trim()}`))
  return [...new Uint8Array(digest)].map(value=>value.toString(16).padStart(2,'0')).join('')
}

export function readPointer(raw:string|null):TaskPointer|null {
  try {const value=JSON.parse(raw??'null');return value&&/^[a-f0-9]{32}$/.test(value.task_id)&&Number.isSafeInteger(value.assignment_id)&&value.assignment_id>0&&Number.isSafeInteger(value.submission_id)&&value.submission_id>0?value:null}
  catch{return null}
}

export async function watchTask(id:string,signal:AbortSignal,onUpdate:(task:GradeTask)=>void,read=homework.task,interval=1000):Promise<void>{
  while(!signal.aborted){
    const task=await read(id,signal)
    if(signal.aborted)return
    onUpdate(task)
    if(terminal(task))return
    await new Promise<void>(resolve=>{
      const done=()=>{clearTimeout(timer);signal.removeEventListener('abort',done);resolve()}
      const timer=setTimeout(done,interval)
      signal.addEventListener('abort',done,{once:true})
      if(signal.aborted)done()
    })
  }
}
