import type { ToolCall } from './mcp-api'
import type { Citation } from './api'
export interface StreamEvent { event:string; data:{text?:string;error?:string;message_id?:number;citations?:Citation[];skill?:{name:string}|null;tool_calls?:ToolCall[];created_at?:string;user_created_at?:string;assistant_created_at?:string} }
// SSE frames may split anywhere, including inside a UTF-8 character or CRLF.
export class SSEParser {
 private pending=''
 push(chunk:string,emit:(v:StreamEvent)=>void){
  this.pending+=chunk
  let m:RegExpExecArray|null
  while((m=/\r?\n\r?\n/.exec(this.pending))){
   const raw=this.pending.slice(0,m.index);this.pending=this.pending.slice(m.index+m[0].length)
   let event='message';const data:string[]=[]
   for(const line of raw.split(/\r?\n/)){if(line.startsWith('event:'))event=line.slice(6).trim();if(line.startsWith('data:'))data.push(line.slice(5).trimStart())}
   if(data.length)emit({event,data:JSON.parse(data.join('\n'))})
  }
 }
}
