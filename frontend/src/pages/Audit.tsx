import {useEffect,useState,type FormEvent} from 'react'
import {request} from '../api'

type AuditEvent={id:number;actor_id:number;time:string;action:string;object:string;result:string;http_status:number;skill_id:string|number|null;duration_ms:number;request_id:string;model:{usage?:{total_tokens?:number;reported?:boolean;complete?:boolean};status?:string}|null;tool_states:Record<string,number>}
type Listing={items:AuditEvent[];page:number;page_size:number;total:number;snapshot_id:number}
const names:Record<string,string>={started:'尚未完成',succeeded:'成功',failed:'失败',denied:'已拒绝',cancelled:'已取消',pending:'等待重试',discarded:'已丢弃'}
export default function AuditPage(){
 const [actor,setActor]=useState(''),[skill,setSkill]=useState(''),[result,setResult]=useState(''),[from,setFrom]=useState(''),[to,setTo]=useState('')
 const [query,setQuery]=useState(''),[page,setPage]=useState(1),[snapshot,setSnapshot]=useState(0),[refresh,setRefresh]=useState(0)
 const [data,setData]=useState<Listing|null>(null),[loading,setLoading]=useState(true),[error,setError]=useState(''),[selected,setSelected]=useState<number|null>(null)
 const [detail,setDetail]=useState<AuditEvent|null>(null),[detailError,setDetailError]=useState(''),[detailRetry,setDetailRetry]=useState(0)
 useEffect(()=>{const c=new AbortController();setLoading(true);setError('');setData(null)
  void request<Listing>(`/api/audit-events?page=${page}${snapshot?`&before_id=${snapshot}`:''}${query}`,{signal:c.signal}).then(v=>{if(!c.signal.aborted)setData(v)}).catch(e=>{if(!c.signal.aborted)setError(e.message||'审计记录读取失败，请重试')}).finally(()=>{if(!c.signal.aborted)setLoading(false)})
  return()=>c.abort()
 },[query,page,snapshot,refresh])
 useEffect(()=>{setDetail(null);setDetailError('');if(selected===null)return;const c=new AbortController();void request<AuditEvent>(`/api/audit-events/${selected}`,{signal:c.signal}).then(v=>{if(!c.signal.aborted)setDetail(v)}).catch(e=>{if(!c.signal.aborted)setDetailError(e.message||'详情读取失败，请重试')});return()=>c.abort()},[selected,detailRetry])
 function search(e:FormEvent){e.preventDefault();const q=new URLSearchParams();if(actor.trim())q.set('actor_id',actor.trim());if(skill.trim())q.set('skill_id',skill.trim());if(result)q.set('result',result);if(from)q.set('from',new Date(from).toISOString());if(to)q.set('to',new Date(to).toISOString());setQuery(q.size?'&'+q.toString():'');setPage(1);setSnapshot(0);setSelected(null);setRefresh(v=>v+1)}
 return <main className="audit-page"><header><p className="eyebrow">班级操作 · 留痕与回查</p><h1>审计记录</h1><p>查看本班操作、技能调用和处理结果。</p></header>
 <form className="audit-filters" onSubmit={search}>
 <label>操作人 ID<input type="number" min="1" step="1" value={actor} onChange={e=>setActor(e.target.value)}/></label>
 <label>技能标识<input maxLength={80} value={skill} onChange={e=>setSkill(e.target.value)} placeholder="如 grading-verdict-v3"/></label>
 <label>开始时间<input type="datetime-local" value={from} onChange={e=>setFrom(e.target.value)}/></label>
 <label>结束时间<input type="datetime-local" value={to} onChange={e=>setTo(e.target.value)}/></label>
 <label>处理结果<select value={result} onChange={e=>setResult(e.target.value)}><option value="">所有结果</option>{Object.entries(names).map(([v,n])=><option value={v} key={v}>{n}</option>)}</select></label>
 <button type="submit" disabled={loading}>查询记录</button></form>
 <p className="muted">未选择时间时显示最近 24 小时，单次查询最多 31 天。时间按当前设备时区显示。</p>
 {selected!==null?<section className="audit-detail"><button onClick={()=>setSelected(null)}>返回记录列表</button>{detailError?<p role="alert">{detailError} <button onClick={()=>setDetailRetry(v=>v+1)}>重试</button></p>:!detail?<p role="status">正在加载详情…</p>:<><h2>操作详情 #{detail.id}</h2><pre>{JSON.stringify(detail,null,2)}</pre></>}</section>:<>
 {loading?<p role="status">正在加载审计记录…</p>:error?<p role="alert">{error} <button onClick={()=>setRefresh(v=>v+1)}>重试</button></p>:data?.items.length===0?<p>该条件下没有审计记录，请调整筛选条件。</p>:<div className="audit-list">{data?.items.map(v=><article key={v.id} className="audit-event"><div><strong>操作人 #{v.actor_id||'未知'}</strong><time>{new Date(v.time).toLocaleString()}</time></div><dl><div><dt>操作</dt><dd>{v.action}</dd></div><div><dt>对象</dt><dd>{v.object}</dd></div><div><dt>结果</dt><dd>{names[v.result]||v.result}{v.http_status?` · ${v.http_status}`:''}</dd></div><div><dt>技能</dt><dd>{v.skill_id??'—'}</dd></div><div><dt>用量 / 耗时</dt><dd>{!v.model?'无模型调用':v.model.usage?.reported===false?'用量未上报':v.model.usage?.total_tokens===undefined?'用量见详情':`${v.model.usage.total_tokens} tokens${v.model.usage.complete===false?'（部分）':''}`} · {v.duration_ms} ms</dd></div></dl><button onClick={()=>setSelected(v.id)}>查看详情</button></article>)}</div>}
 <div className="audit-pagination"><button disabled={loading||page===1} onClick={()=>{setSnapshot(data?.snapshot_id||snapshot);setPage(v=>v-1)}}>上一页</button><span>第 {page} 页{data?` · ${data.total} 条`:''}</span><button disabled={loading||!data||page*data.page_size>=data.total} onClick={()=>{setSnapshot(data!.snapshot_id);setPage(v=>v+1)}}>下一页</button></div></>}
 </main>
}
