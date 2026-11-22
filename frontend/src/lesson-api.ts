import {request,type Citation} from './api'
import type {Subject} from './subjects'
import type {Page} from './grading'

export interface LessonInput {subject:Subject;chapter:string;objectives:string;periods:number;minutes_per_period:number;material_ids:number[];client_key:string}
export interface LessonSummary {id:number;subject:Subject;title:string;version:number;created_at:string;updated_at:string}
export interface LessonPlan extends LessonSummary {input:LessonInput;body:string;citations:(Citation&{chunk_id:number;available:boolean})[]}
export interface LessonGeneration {status:'generated'|'insufficient_evidence';message?:string;draft:LessonPlan|null}
const json=(method:string,body:unknown,signal?:AbortSignal):RequestInit=>({method,headers:{'Content-Type':'application/json'},body:JSON.stringify(body),signal})
export const lessonAPI={
 list:(subject:Subject,page:number,q:string,signal?:AbortSignal)=>request<Page<LessonSummary>>(`/api/lesson-plans?subject=${subject}&page=${page}&q=${encodeURIComponent(q)}`,{signal}),
 get:(id:number,signal?:AbortSignal)=>request<LessonPlan>(`/api/lesson-plans/${id}`,{signal}),
 generate:(input:LessonInput,signal:AbortSignal)=>request<LessonGeneration>('/api/lesson-plans/generate',json('POST',input,signal)),
 save:(id:number,title:string,body:string,version:number)=>request<LessonPlan>(`/api/lesson-plans/${id}`,json('PUT',{title,body,expected_version:version})),
 remove:(id:number,version:number)=>request<void>(`/api/lesson-plans/${id}`,json('DELETE',{expected_version:version})),
}
