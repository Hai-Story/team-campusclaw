import {request,getToken,clearToken} from './api'
import {SSEParser} from './sse'
import type {Subject} from './subjects'
import type {Page} from './grading'
export type WalkStage='read'|'method'|'derive'|'check'|'completed'
export const stageNames:Record<WalkStage,string>={read:'审题',method:'选方法',derive:'推导',check:'检查',completed:'已完成'}
export interface WalkTurn {id:number;content:string;action:string;stage_before:WalkStage;stage_after:WalkStage;status:'pending'|'succeeded'|'failed'|'cancelled';response:string;summary:string;improvements:string[]}
export interface Walk {id:number;assignment_id:number;subject:Subject;title:string;question:string;stage:WalkStage;version:number;summary:string;improvements:string[];turns:WalkTurn[];active:boolean;created_at:string;updated_at:string}
export const walkAPI={list:(subject:Subject,page:number,signal?:AbortSignal)=>request<Page<Walk>>(`/api/walkthroughs?subject=${subject}&page=${page}`,{signal}),get:(id:number,signal?:AbortSignal)=>request<Walk>('/api/walkthroughs/'+id,{signal}),create:(assignment_id:number,client_key:string)=>request<Walk>('/api/walkthroughs',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({assignment_id,client_key})})}
export async function walkStream(id:number,input:{content:string;action:string;expected_version:number;client_key:string},signal:AbortSignal){
 const res=await fetch(`/api/walkthroughs/${id}/turns`,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${getToken()}`},body:JSON.stringify(input),signal})
 if(res.status===401){clearToken();window.dispatchEvent(new Event('tutor-unauthorized'));throw new Error('请重新登录')}
 if(!res.ok){const e=await res.json();throw new Error(e.error??'讲题暂不可用，请重试')}
 if(!res.body)throw new Error('浏览器无法读取生成过程')
 const reader=res.body.getReader(),decoder=new TextDecoder(),parser=new SSEParser();let done=false,problem=''
 const consume=(v:{event:string;data:{error?:string}})=>{if(v.event==='done')done=true;if(v.event==='error')problem=v.data.error||'讲题失败，请重试'}
 try{while(true){const v=await reader.read();if(v.done)break;parser.push(decoder.decode(v.value,{stream:true}),consume)}parser.push(decoder.decode(),consume);if(problem)throw new Error(problem);if(!done)throw new Error('连接中断，请刷新恢复记录')}finally{reader.releaseLock()}
}
