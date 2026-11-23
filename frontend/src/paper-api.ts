import {request} from './api'
import type {Subject} from './subjects'
import type {Page,Submission} from './grading'
import type {QuestionKind,Sources} from './bank-api'
export interface PublishedQuestion{position:number;assignment_id:number;score:number;type:QuestionKind;stem:string;options:string[];knowledge_point_ids:number[];answer?:string[];explanation?:string;citations?:Sources}
export interface PublishedPaper{id:number;subject:Subject;title:string;total_score:number;question_count:number;created_at:string;questions?:PublishedQuestion[]}
export interface PaperAnswer{assignment_id:number;answer:string}
export interface PaperAttempt{id:number;paper_id:number;student_id:number;student_name:string;created_at:string;latest:boolean;reviewed_count:number;reviewed_score:number;final_score:number|null;total_score:number;items:Submission[]}
const json=(method:string,body:unknown):RequestInit=>({method,headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})
export const paperAPI={
 publish:(id:number,version:number)=>request<PublishedPaper>(`/api/paper-drafts/${id}/publish`,json('POST',{expected_version:version})),
 list:(subject:Subject,page:number,signal?:AbortSignal)=>request<Page<PublishedPaper>>(`/api/published-papers?subject=${subject}&page=${page}`,{signal}),
 get:(id:number,signal?:AbortSignal)=>request<PublishedPaper>('/api/published-papers/'+id,{signal}),
 attempts:(id:number,page:number,signal?:AbortSignal)=>request<Page<PaperAttempt>>(`/api/published-papers/${id}/attempts?page=${page}`,{signal}),
 attempt:(id:number,signal?:AbortSignal)=>request<PaperAttempt>('/api/paper-attempts/'+id,{signal}),
 submit:(id:number,answers:PaperAnswer[],key:string)=>request<PaperAttempt>(`/api/published-papers/${id}/attempts`,{...json('POST',{answers}),headers:{'Content-Type':'application/json','Idempotency-Key':key}}),
}
