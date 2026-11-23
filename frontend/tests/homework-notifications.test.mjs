import assert from 'node:assert/strict'
import {watchNotices} from '../src/homework-notifications.ts'
const flush=()=>new Promise(resolve=>setImmediate(resolve))
function fixture(){
 let visible=true,listener,tick,timeout;const pending=[],seen=[],errors=[]
 const env={visible:()=>visible,subscribe:fn=>{listener=fn;return()=>listener=null},interval:(fn,ms)=>{assert.equal(ms,30000);tick=fn;return()=>tick=null},timeout:(fn,ms)=>{assert.equal(ms,15000);timeout=fn;return()=>{if(timeout===fn)timeout=null}}}
 const dispose=watchNotices(signal=>new Promise((resolve,reject)=>pending.push({signal,resolve,reject})),v=>seen.push(v),()=>errors.push('error'),env)
 return {pending,seen,errors,dispose,focus:()=>listener?.(),tick:()=>tick?.(),timeout:()=>timeout?.(),hide:()=>{visible=false;listener?.()},show:()=>{visible=true;listener?.()},cleared:()=>!listener&&!tick}
}
const data={items:[],page:1,page_size:20,unread_count:3}
const a=fixture();assert.equal(a.pending.length,1);a.focus();a.tick();assert.equal(a.pending.length,1)
a.pending[0].resolve(data);await flush();assert.deepEqual(a.seen,[data]);a.tick();assert.equal(a.pending.length,2)
a.pending[1].reject(new Error('offline'));await flush();assert.equal(a.errors.length,1);assert.deepEqual(a.seen,[data])
a.focus();a.pending[2].resolve({...data,unread_count:4});await flush();assert.equal(a.seen[1].unread_count,4)
a.dispose();assert.ok(a.cleared());a.tick();a.focus();assert.equal(a.pending.length,3)
const b=fixture();b.hide();assert.ok(b.pending[0].signal.aborted);b.tick();b.pending[0].resolve(data);await flush();assert.equal(b.seen.length,0);assert.equal(b.errors.length,0)
b.show();assert.equal(b.pending.length,2);b.dispose();assert.ok(b.pending[1].signal.aborted);b.pending[1].resolve(data);await flush();assert.equal(b.seen.length,0)
const c=fixture();c.timeout();assert.ok(c.pending[0].signal.aborted);assert.equal(c.errors.length,1);c.pending[0].reject(new DOMException('timeout','AbortError'));await flush();assert.equal(c.errors.length,1);c.tick();assert.equal(c.pending.length,2);c.dispose()
const d=fixture();d.dispose();d.pending[0].reject(new Error('late failure'));await flush();assert.equal(d.errors.length,0)
console.log('PASS N3: initial/focus/30s refresh, no overlapping reads, hidden/dispose cancellation, timeout, failure preserves state and next refresh recovers')
