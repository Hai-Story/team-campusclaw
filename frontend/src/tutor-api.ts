import { request, getToken, clearToken, type Citation } from './api'
import type { ToolCall } from './mcp-api'
import { SSEParser, type StreamEvent } from './sse'
export interface Agent { id:number; name:string; prompt:string; enabled:boolean; skill_ids:number[]; material_ids:number[] }
export interface Skill { id:number; name:string; source_text:string; enabled:boolean; trigger_text:string;tools:string[];resources?:Record<string,string> }
export interface Conversation { id:number; agent_id:number; agent_name:string; agent_deleted:boolean; title:string;created_at:string;updated_at:string }
export interface Message { id:number; role:'user'|'assistant'; content:string; citations:Citation[]; skill_name:string; status:'complete'|'partial';created_at:string;has_trace?:boolean;tool_calls?:ToolCall[] }
export interface LLMTrace {truncated:boolean;calls:{phase:string;output_text?:string;output_truncated:boolean;finish_reason?:string;usage?:Record<string,unknown>;request?:Record<string,unknown>;request_truncated:boolean;response:string;response_truncated:boolean;http_status:number;started_at:string;error?:string}[]}
export const tutorAPI={
 trace:(cid:number,mid:number)=>request<LLMTrace>(`/api/conversations/${cid}/messages/${mid}/trace`),
 importSkill:(file:File)=>{const data=new FormData();data.append('file',file);return request<Skill>('/api/skills/import',{method:'POST',body:data})},
 agents:()=>request<Agent[]>('/api/agents'),
 saveAgent:(a:Omit<Agent,'id'>,id?:number)=>request<Agent>(id?`/api/agents/${id}`:'/api/agents',{method:id?'PUT':'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(a)}),
 deleteAgent:(id:number)=>request<void>(`/api/agents/${id}`,{method:'DELETE'}),
 skills:()=>request<Skill[]>('/api/skills'),
 saveSkill:(source_text:string,id?:number)=>request<Skill>(id?`/api/skills/${id}`:'/api/skills',{method:id?'PUT':'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({source_text})}),
 deleteSkill:(id:number)=>request<void>(`/api/skills/${id}`,{method:'DELETE'}),
 deleteConversation:(id:number)=>request<void>(`/api/conversations/${id}`,{method:'DELETE'}),
 conversations:()=>request<Conversation[]>('/api/conversations'),
 create:(agent_id:number)=>request<Conversation>('/api/conversations',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({agent_id})}),
 conversation:(id:number)=>request<{conversation:Conversation;messages:Message[]}>(`/api/conversations/${id}`),
}
export async function sendStream(id:number,content:string,signal:AbortSignal,emit:(v:StreamEvent)=>void,captureTrace=false){
 const res=await fetch(`/api/conversations/${id}/messages`,{method:'POST',headers:{'Content-Type':'application/json',Authorization:`Bearer ${getToken()}`},body:JSON.stringify({content,capture_trace:captureTrace}),signal})
 if(res.status===401){clearToken();window.dispatchEvent(new Event('tutor-unauthorized'));throw new Error('未登录')}
 if(!res.ok){const e=await res.json();throw new Error(e.error??`请求失败 ${res.status}`)}
 if(!res.body)throw new Error('浏览器不支持流式读取')
 const reader=res.body.getReader();const decoder=new TextDecoder();const parser=new SSEParser();let terminal=false
 const consume=(v:StreamEvent)=>{if(v.event==='done'||v.event==='error')terminal=true;emit(v)}
 try{while(true){const {value,done}=await reader.read();if(done)break;parser.push(decoder.decode(value,{stream:true}),consume)}parser.push(decoder.decode(),consume);if(!terminal)throw new Error('连接中断，回答可能未完成')}finally{reader.releaseLock()}
}
