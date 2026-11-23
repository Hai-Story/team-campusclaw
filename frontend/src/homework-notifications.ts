import {request} from './api.ts'
import type {Subject} from './subjects.ts'
export type HomeworkNotice={id:number;kind:'assignment'|'paper';resource_id:number;subject:Subject;title:string;created_at:string;read_at:string|null}
export type NoticePage={items:HomeworkNotice[];page:number;page_size:number;unread_count:number}
export const noticesAPI={list:(page:number,signal:AbortSignal)=>request<NoticePage>(`/api/homework-notifications?page=${page}`,{signal}),read:(id:number,signal:AbortSignal)=>request<void>(`/api/homework-notifications/${id}/read`,{method:'PUT',signal})}

type PollEnvironment={visible:()=>boolean;subscribe:(run:()=>void)=>()=>void;interval:(run:()=>void,ms:number)=>()=>void;timeout:(run:()=>void,ms:number)=>()=>void}
export const browserPolling:PollEnvironment={visible:()=>document.visibilityState!=='hidden',subscribe:run=>{window.addEventListener('focus',run);document.addEventListener('visibilitychange',run);return()=>{window.removeEventListener('focus',run);document.removeEventListener('visibilitychange',run)}},interval:(run,ms)=>{const id=setInterval(run,ms);return()=>clearInterval(id)},timeout:(run,ms)=>{const id=setTimeout(run,ms);return()=>clearTimeout(id)}}
// One in-flight request per account; hidden pages pause and dispose cancels late writes.
export function watchNotices(load:(signal:AbortSignal)=>Promise<NoticePage>,success:(page:NoticePage)=>void,failure:()=>void,env:PollEnvironment=browserPolling){
 let active=true,pending:AbortController|null=null
 async function run(){
  if(!env.visible()){pending?.abort();return}
  if(!active||pending)return
  const c=new AbortController();pending=c;const clear=env.timeout(()=>{c.abort();if(active&&env.visible())failure()},15000)
  try{const value=await load(c.signal);if(active&&!c.signal.aborted)success(value)}catch{if(active&&!c.signal.aborted)failure()}finally{clear();if(pending===c)pending=null}
 }
 const unsubscribe=env.subscribe(()=>void run()),clear=env.interval(()=>void run(),30000);void run()
 return()=>{active=false;pending?.abort();unsubscribe();clear()}
}
