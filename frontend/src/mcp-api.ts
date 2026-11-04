import { request } from './api'
export interface MCPServer { id:number;name:string;base_url:string;connected:boolean;has_auth:boolean;last_error:string;tool_count:number }
export interface MCPTool {name:string;description:string;inputSchema:Record<string,unknown>;annotations:Record<string,unknown>;fingerprint:string;approved:boolean;read_only:boolean}
export interface ToolCall {id?:number;ref:string;status:'ok'|'denied'|'error';text:string;round:number;server_name?:string;username?:string;user_id?:number;conversation_id?:number;request_message_id?:number;created_at?:string}
const json=(body:unknown,method='POST')=>({method,headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})
export const mcpAPI={
 servers:()=>request<MCPServer[]>('/api/mcp/servers'),
 save:(v:{name:string;base_url:string;auth_token:string},id?:number)=>request<MCPServer>(id?`/api/mcp/servers/${id}`:'/api/mcp/servers',json(v,id?'PUT':'POST')),
 connect:(id:number)=>request<MCPServer>(`/api/mcp/servers/${id}/connect`,json({})),
 disconnect:(id:number)=>request<MCPServer>(`/api/mcp/servers/${id}/disconnect`,json({})),
 remove:(id:number)=>request<void>(`/api/mcp/servers/${id}`,{method:'DELETE'}),
 tools:(id:number)=>request<MCPTool[]>(`/api/mcp/servers/${id}/tools`),
 approve:(id:number,t:MCPTool)=>request<{approved:boolean}>(`/api/mcp/servers/${id}/tools/${encodeURIComponent(t.name)}/approve`,json({fingerprint:t.fingerprint})),
 calls:()=>request<ToolCall[]>('/api/mcp/tool-calls'),
}
