import {useEffect,useState} from 'react'
import {availableBuild} from '../build-update'
declare const __APP_BUILD_ID__:string

export default function UpdateNotice(){
 const [next,setNext]=useState<string|null>(null),[dismissed,setDismissed]=useState<string|null>(null)
 useEffect(()=>{
  let active=true,pending:AbortController|null=null
  async function check(){
   if(pending||document.visibilityState==='hidden')return
   const c=new AbortController();pending=c;const timer=setTimeout(()=>c.abort(),5000)
   try{const build=await availableBuild(__APP_BUILD_ID__,c.signal);if(active&&!c.signal.aborted)setNext(build)}catch{/* Retry on focus or the next timer; business requests remain unaffected. */}
   finally{clearTimeout(timer);if(pending===c)pending=null}
  }
  void check();const timer=setInterval(()=>void check(),60000)
  const focus=()=>void check();window.addEventListener('focus',focus);document.addEventListener('visibilitychange',focus)
  return()=>{active=false;clearInterval(timer);pending?.abort();window.removeEventListener('focus',focus);document.removeEventListener('visibilitychange',focus)}
 },[])
 if(!next||next===dismissed)return null
 return <aside className="update-notice" role="status"><span>有新版本可用，请保存当前编辑后刷新。</span><button onClick={()=>location.reload()}>刷新页面</button><button onClick={()=>setDismissed(next)}>稍后</button></aside>
}
