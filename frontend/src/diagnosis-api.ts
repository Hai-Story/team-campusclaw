import {request} from './api'
import type {Subject} from './subjects'
export interface LearningStats{samples:number;correct:number;score:number;max_score:number;score_percent:number|null;correct_percent:number|null}
export interface DiagnosisSample{submission_id:number;assignment_id:number;student_id:number;student_name:string;title:string;score:number;max_score:number;correct:boolean;review_version:number;submitted_at:string;reviewed_at:string;error_tags:string[];knowledge_point_ids:number[];question?:string;answer?:string;feedback?:string}
export interface DiagnosisReport extends LearningStats{subject:Subject;from:string;to:string;timezone:string;fingerprint:string;student_count:number;students:{id:number;name:string}[];points:({id:number;name:string}&LearningStats)[];error_counts:Record<string,number>;trend:({date:string}&LearningStats)[];evidence:DiagnosisSample[];page:number;page_size:number;total_evidence:number}
export interface DiagnosisScope{subject:Subject;days:number;student_id?:number}
export const diagnosisAPI={
 report:(scope:DiagnosisScope,page:number,signal?:AbortSignal)=>request<DiagnosisReport>('/api/learning-diagnosis?'+new URLSearchParams(Object.entries({...scope,page}).map(([k,v])=>[k,String(v)])),{signal}),
 sample:(id:number,signal?:AbortSignal)=>request<DiagnosisSample>('/api/learning-diagnosis/samples/'+id,{signal}),
 explain:(scope:DiagnosisScope,fingerprint:string,signal:AbortSignal)=>request<{status:string;explanation:string;fingerprint:string}>('/api/learning-diagnosis/explanation',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...scope,fingerprint}),signal})
}
