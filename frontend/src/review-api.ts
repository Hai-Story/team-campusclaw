import {request} from './api'
import type {Subject} from './subjects'
import type {Page} from './grading'
export interface ReviewSettings{intervals:number[];version:number}
export interface ReviewCard{id:number;source_kind:'mistake'|'favorite';source_id:number;subject:Subject;title:string;body:string;level:number;due_at:string;version:number;active:boolean;assignment_id?:number}
export interface ReviewCompletion{id:number;card_id:number;feedback:'good'|'again';previous_level:number;level:number;due_at:string;completed_at:string;card:ReviewCard}
export interface ReviewReport{subject:Subject;items:ReviewCard[];total:number;due_count:number;settings:ReviewSettings;server_time:string;timezone:string;page:number;page_size:number}
const json=(method:string,body:unknown):RequestInit=>({method,headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})
export const reviewAPI={
 sync:(signal?:AbortSignal)=>request<{status:string}>('/api/review-cards/sync',{...json('POST',{}),signal}),
 count:(signal?:AbortSignal)=>request<{due_count:number;server_time:string}>('/api/review-cards/count',{signal}),
 list:(subject:Subject,status:'due'|'all',page:number,signal?:AbortSignal)=>request<ReviewReport>(`/api/review-cards?subject=${subject}&status=${status}&page=${page}`,{signal}),
 settings:(signal?:AbortSignal)=>request<ReviewSettings>('/api/review-settings',{signal}),
 saveSettings:(intervals:number[],expected_version:number,signal?:AbortSignal)=>request<ReviewSettings>('/api/review-settings',{...json('PUT',{intervals,expected_version}),signal}),
 complete:(card:ReviewCard,feedback:'good'|'again',client_key:string,signal?:AbortSignal)=>request<ReviewCompletion>(`/api/review-cards/${card.id}/complete`,{...json('POST',{feedback,client_key,expected_version:card.version}),signal}),
 history:(subject:Subject,page:number,signal?:AbortSignal)=>request<Page<ReviewCompletion>>(`/api/review-completions?subject=${subject}&page=${page}`,{signal}),
}
