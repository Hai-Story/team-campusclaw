import {request} from './api'
import type {Subject} from './subjects'
import type {Page} from './grading'
import type {LessonPlan} from './lesson-api'
export type QuestionKind='single_choice'|'multiple_choice'|'true_false'|'short_answer'
export type Difficulty='easy'|'medium'|'hard'
export const questionKinds:Record<QuestionKind,string>={single_choice:'单选题',multiple_choice:'多选题',true_false:'判断题',short_answer:'简答题'}
export const difficulties:Record<Difficulty,string>={easy:'基础',medium:'适中',hard:'提高'}
export type Sources=LessonPlan['citations']
export interface QuestionItem{type:QuestionKind;stem:string;options:string[];answer:string[];explanation:string;knowledge_point_ids:number[];difficulty:Difficulty;citation_indices:number[]}
export interface BankConfig{subject:Subject;knowledge_point_ids:number[];question_types:QuestionKind[];difficulty:Difficulty;question_count:number;material_ids:number[]}
export interface QuestionDraft{id:number;subject:Subject;input:BankConfig;questions:QuestionItem[];citations:Sources;bank_ids:number[];status:'pending'|'approved';version:number;created_at:string}
export interface BankQuestion{id:number;subject:Subject;question:QuestionItem;citations:Sources;version:number;created_at:string;updated_at:string}
export interface PaperSelection{question_id:number;question_version:number;score:number}
export interface PaperItem extends PaperSelection{question:QuestionItem;citations:Sources}
export interface PaperSummary{id:number;subject:Subject;title:string;total_score:number;version:number;created_at:string;updated_at:string}
export interface PaperDraft extends PaperSummary{items:PaperItem[]}
const json=(method:string,body:unknown):RequestInit=>({method,headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})
const list=<T,>(resource:string,subject:Subject,page:number,q:string,signal?:AbortSignal)=>request<Page<T>>(`/api/${resource}?subject=${subject}&page=${page}&q=${encodeURIComponent(q)}`,{signal})
export const bankAPI={
 generate:(input:BankConfig,client_key:string,signal:AbortSignal)=>request<{status:string;message?:string;draft:QuestionDraft|null}>('/api/question-drafts/generate',{...json('POST',{...input,client_key}),signal}),
 drafts:(subject:Subject,page:number,signal?:AbortSignal)=>list<QuestionDraft>('question-drafts',subject,page,'',signal),draft:(id:number,signal?:AbortSignal)=>request<QuestionDraft>('/api/question-drafts/'+id,{signal}),
 saveDraft:(d:QuestionDraft,questions:QuestionItem[])=>request<QuestionDraft>('/api/question-drafts/'+d.id,json('PUT',{questions,expected_version:d.version})),approve:(d:QuestionDraft)=>request<QuestionDraft>(`/api/question-drafts/${d.id}/approve`,json('POST',{expected_version:d.version})),
 questions:(subject:Subject,page:number,q:string,signal?:AbortSignal)=>list<BankQuestion>('bank-questions',subject,page,q,signal),question:(id:number,signal?:AbortSignal)=>request<BankQuestion>('/api/bank-questions/'+id,{signal}),
 saveQuestion:(q:BankQuestion,question:QuestionItem)=>request<BankQuestion>('/api/bank-questions/'+q.id,json('PUT',{question,expected_version:q.version,confirm_review:true})),removeQuestion:(q:BankQuestion)=>request<void>('/api/bank-questions/'+q.id,json('DELETE',{expected_version:q.version})),
 papers:(subject:Subject,page:number,q:string,signal?:AbortSignal)=>list<PaperSummary>('paper-drafts',subject,page,q,signal),paper:(id:number,signal?:AbortSignal)=>request<PaperDraft>('/api/paper-drafts/'+id,{signal}),
 createPaper:(subject:Subject,title:string,items:PaperSelection[],client_key:string)=>request<PaperDraft>('/api/paper-drafts',json('POST',{subject,title,items,client_key})),savePaper:(p:PaperDraft,title:string,items:PaperSelection[])=>request<PaperDraft>('/api/paper-drafts/'+p.id,json('PUT',{title,items,expected_version:p.version})),
}
export function answerText(q:QuestionItem){return q.answer.map(a=>q.type==='true_false'?(a==='true'?'正确':'错误'):a).join('、')}
