import type { Message } from './tutor-api'
import type { StreamEvent } from './sse'
export function applyMessageEvent(m:Message,v:StreamEvent):Message {
 if(v.event==='meta')return {...m,
  ...(v.data.assistant_created_at?{created_at:v.data.assistant_created_at}:{}),
  ...(v.data.citations!==undefined?{citations:v.data.citations}:{}),
  ...(v.data.skill!==undefined?{skill_name:v.data.skill?.name??''}:{}),
  ...(v.data.tool_calls?{tool_calls:[...(m.tool_calls??[]),...v.data.tool_calls.filter(c=>!c.id||!(m.tool_calls??[]).some(old=>old.id===c.id))]}:{}),
 }
 if(v.event==='delta')return {...m,content:m.content+(v.data.text??'')}
 if(v.event==='done')return {...m,id:v.data.message_id??m.id,status:'complete',created_at:v.data.created_at??m.created_at}
 return m
}
