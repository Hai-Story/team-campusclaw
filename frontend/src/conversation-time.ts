import type { Conversation } from './tutor-api'

function instant(value?:string):number|null {
 if(!value||!/(?:Z|[+-]\d{2}:\d{2})$/i.test(value))return null
 const ms=Date.parse(value)
 return Number.isFinite(ms)?ms:null
}
const beijing=new Intl.DateTimeFormat('zh-CN',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hourCycle:'h23'})
export function absoluteTime(value?:string):string {
 const ms=instant(value)
 if(ms===null)return ''
 const p=Object.fromEntries(beijing.formatToParts(ms).map(p=>[p.type,p.value]))
 return `${p.year}年${p.month}月${p.day}日${p.hour}:${p.minute}`
}
export function relativeTime(value:string|undefined,now=Date.now()):string {
 const ms=instant(value)
 if(ms===null)return '时间未知'
 const seconds=Math.max(0,Math.floor((now-ms)/1000))
 if(seconds<60)return '刚刚'
 for(const [size,label] of [[31536000,'年'],[2592000,'个月'],[86400,'天'],[3600,'小时'],[60,'分钟']] as const){
  if(seconds>=size)return `${Math.floor(seconds/size)}${label}前`
 }
 return '刚刚'
}
export function touchConversation(items:Conversation[],id:number,at?:string):Conversation[] {
 const ms=instant(at)
 if(ms===null)return items
 return items.map(c=>c.id===id&&ms>=(instant(c.updated_at)??0)?{...c,updated_at:at!}:c)
  .sort((a,b)=>(instant(b.updated_at)??0)-(instant(a.updated_at)??0)||b.id-a.id)
}
