import { request } from './api'
import type { Subject } from './subjects'
import type { Page } from './grading'

export interface KnowledgePoint {
 id:number;subject:Subject;name:string;chapter:string;description:string;version:number;
 material_ids:number[];assignment_ids:number[];favorite:boolean;created_at:string;updated_at:string
}
export interface PointDraft {subject:Subject;name:string;chapter:string;description:string;material_ids:number[];assignment_ids:number[];expected_version?:number}
const json=(method:string,body:unknown):RequestInit=>({method,headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})
export const knowledgeAPI={
 list:(subject:Subject,page:number,q:string,favorites:boolean,signal?:AbortSignal)=>request<Page<KnowledgePoint>>(`/api/knowledge-points?subject=${subject}&page=${page}&q=${encodeURIComponent(q)}&favorites=${favorites}`,{signal}),
 get:(id:number,signal?:AbortSignal)=>request<KnowledgePoint>(`/api/knowledge-points/${id}`,{signal}),
 save:(input:PointDraft,id?:number)=>request<KnowledgePoint>(id?`/api/knowledge-points/${id}`:'/api/knowledge-points',json(id?'PUT':'POST',input)),
 remove:(id:number,version:number)=>request<void>(`/api/knowledge-points/${id}`,json('DELETE',{expected_version:version})),
 favorite:(id:number,value:boolean)=>request<void>(`/api/knowledge-points/${id}/favorite`,{method:value?'PUT':'DELETE'}),
}
