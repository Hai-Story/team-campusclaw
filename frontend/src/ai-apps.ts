import { request } from './api'
import type { Conversation } from './tutor-api'
export interface AIApp {key:string;name:string;description:string;icon:string;system_template:string;user_template:string;template_version:number;fields:{key:string;label:string;placeholder:string;max_length:number}[];agent_id:number;enabled:boolean}
export interface AppLaunch {conversation:Conversation;prompt:string}
export const appsAPI={saveTemplate:(key:string,user_template:string,expected_version:number)=>request<AIApp>(`/api/ai-apps/${key}/template`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({user_template,expected_version})}),list:()=>request<AIApp[]>('/api/ai-apps'),launch:(key:string,fields:Record<string,string>)=>request<AppLaunch>(`/api/ai-apps/${key}/launch`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({fields})})}
