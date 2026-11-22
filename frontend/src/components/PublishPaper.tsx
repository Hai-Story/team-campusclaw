import {useEffect,useRef,useState} from 'react'
import {paperAPI,type PublishedPaper} from '../paper-api'
import type {PaperDraft} from '../bank-api'
export default function PublishPaper({paper,onPublished,onBusy}:{paper:PaperDraft;onPublished:(p:PublishedPaper)=>void;onBusy:(v:boolean)=>void}){
 const [confirm,setConfirm]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState('');const mounted=useRef(true)
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false}},[])
 async function publish(){if(busy)return;setBusy(true);onBusy(true);setError('');try{const p=await paperAPI.publish(paper.id,paper.version);if(mounted.current){setConfirm(false);onPublished(p)}}catch(e){if(mounted.current)setError(e instanceof Error?e.message:'发布失败，请刷新草稿核对后重试')}finally{onBusy(false);if(mounted.current)setBusy(false)}}
 return <div className="paper-publish">{!confirm?<button className="primary" disabled={busy} onClick={()=>setConfirm(true)}>发布为多题作业</button>:<div role="group" aria-label="确认发布试卷"><p>将此已保存版本发布到本班，题面与分值固定保存。</p><div className="toolbar"><button disabled={busy} onClick={()=>{setConfirm(false);setError('')}}>取消发布</button><button className="primary" disabled={busy} onClick={()=>void publish()}>{busy?'正在发布…':'确认发布'}</button></div></div>}{error&&<p role="alert" className="form-error">{error}</p>}</div>
}
