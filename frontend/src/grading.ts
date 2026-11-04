import { request } from './api.ts'

export interface Assignment { id:number; title:string; question:string; expected_answer?:string; max_score:number }
export interface GradeTask {
  task_id:string; submission_id:number; status:'pending'|'running'|'succeeded'|'failed'; attempts:number; error?:string;
  result?:{score:number;max_score:number;feedback:string}
}
export interface TaskPointer { task_id:string; assignment_id:number; submission_id:number }
export const terminal = (task:GradeTask) => task.status==='succeeded'||task.status==='failed'
export const homework = {
  list:(page:number,signal?:AbortSignal)=>request<{items:Assignment[];page:number;page_size:number}>(`/api/assignments?page=${page}`,{signal}),
  create:(input:Omit<Assignment,'id'>)=>request<Assignment>('/api/assignments',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(input)}),
  submit:(id:number,answer:string,key:string)=>request<Pick<GradeTask,'task_id'|'submission_id'|'status'>&{status_url:string}>(`/api/assignments/${id}/submissions`,{method:'POST',headers:{'Content-Type':'application/json','Idempotency-Key':key},body:JSON.stringify({answer})}),
  task:(id:string,signal:AbortSignal)=>request<GradeTask>('/api/tasks/'+id,{signal}),
}

export async function submissionKey(user:number,assignment:number,answer:string):Promise<string>{
  const digest=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(`${user}:${assignment}:${answer.trim()}`))
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
