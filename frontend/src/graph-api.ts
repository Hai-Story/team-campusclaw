import {request} from './api'
import type {Subject} from './subjects'
import type {Page} from './grading'
import type {KnowledgePoint} from './knowledge-api'
import type {LessonPlan} from './lesson-api'
export type Relation='prerequisite'|'contains'|'related'
export const relations:Record<Relation,string>={prerequisite:'先修',contains:'包含',related:'关联'}
export interface Mastery{reviewed:number;incorrect:number;weak:boolean}
export interface GraphNode extends Mastery{id:number;name:string;chapter:string}
export interface GraphEdge{id:number;from_id:number;to_id:number;kind:Relation;version:number}
export interface GraphData{subject:Subject;nodes:GraphNode[];edges:GraphEdge[]}
export interface GraphCandidate{nodes:{name:string;chapter:string;description:string}[];edges:{from:string;to:string;kind:Relation}[];citation_indices:number[]}
export interface GraphDraft extends GraphCandidate{id:number;subject:Subject;existing_nodes:Record<string,number>;citations:LessonPlan['citations'];point_ids:number[];status:'pending'|'approved';version:number;created_at:string}
export interface NodeResources extends Mastery{point:KnowledgePoint;materials:{id:number;title:string}[];assignments:{id:number;title:string}[];recommended_points:GraphNode[];review_materials:{id:number;title:string}[];mistakes:{submission_id:number;assignment_id:number;title:string;answer:string}[]}
const json=(method:string,body:unknown):RequestInit=>({method,headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})
export const graphAPI={get:(subject:Subject,signal?:AbortSignal)=>request<GraphData>('/api/knowledge-graph?subject='+subject,{signal}),resources:(id:number,signal?:AbortSignal)=>request<NodeResources>(`/api/knowledge-graph/nodes/${id}/resources`,{signal}),edge:(from_id:number,to_id:number,kind:Relation)=>request<GraphEdge>('/api/knowledge-graph/edges',json('POST',{from_id,to_id,kind})),remove:(e:GraphEdge)=>request<void>('/api/knowledge-graph/edges/'+e.id,json('DELETE',{expected_version:e.version})),generate:(subject:Subject,material_ids:number[],client_key:string,signal:AbortSignal)=>request<{status:string;message?:string;draft:GraphDraft|null}>('/api/knowledge-graph/drafts/generate',{...json('POST',{subject,material_ids,client_key}),signal}),drafts:(subject:Subject,page:number,signal?:AbortSignal)=>request<Page<GraphDraft>>(`/api/knowledge-graph/drafts?subject=${subject}&page=${page}`,{signal}),draft:(id:number,signal?:AbortSignal)=>request<GraphDraft>('/api/knowledge-graph/drafts/'+id,{signal}),save:(d:GraphDraft,edit:GraphCandidate)=>request<GraphDraft>('/api/knowledge-graph/drafts/'+d.id,json('PUT',{...edit,expected_version:d.version})),approve:(d:GraphDraft)=>request<GraphDraft>(`/api/knowledge-graph/drafts/${d.id}/approve`,json('POST',{expected_version:d.version}))}
