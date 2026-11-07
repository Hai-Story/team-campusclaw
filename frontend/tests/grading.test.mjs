import assert from 'node:assert/strict'
import { submissionKey, watchTask, readPointer, effectiveCorrect, readSubject } from '../src/grading.ts'

const key=await submissionKey(1,2,' 2 ','attempt-1')
assert.equal(key,await submissionKey(1,2,'2','attempt-1'))
assert.notEqual(key,await submissionKey(1,2,'3','attempt-1'))
assert.notEqual(key,await submissionKey(2,2,'2','attempt-1'))
assert.equal(key.length,64)
// A12-S1: a new deliberate attempt is independent even when its answer is unchanged.
assert.notEqual(key,await submissionKey(1,2,'2','attempt-2'))
assert.equal(readPointer('{broken'),null)
assert.equal(readPointer(JSON.stringify({task_id:'../x',assignment_id:1,submission_id:1})),null)
assert.ok(readPointer(JSON.stringify({task_id:'a'.repeat(32),assignment_id:1,submission_id:1})))
let reads=0;const seen=[];const control=new AbortController()
await watchTask('task',control.signal,value=>seen.push(value.status),async()=>({status:['pending','running','succeeded'][reads++]}),1)
assert.deepEqual(seen,['pending','running','succeeded']);assert.equal(reads,3)
let finish;const cancelled=new AbortController(),updates=[]
const waiting=watchTask('old',cancelled.signal,value=>updates.push(value),()=>new Promise(resolve=>finish=resolve),1)
cancelled.abort();finish({status:'succeeded'});await waiting;assert.equal(updates.length,0)
let attempts=0;const failure=new AbortController()
await assert.rejects(watchTask('task',failure.signal,()=>{},async()=>{attempts++;throw new Error('controlled network failure')},1),/controlled/)
assert.equal(attempts,1)
console.log('PASS: business idempotency, pointer validation, terminal polling and stale request cancellation')

// A8: a teacher's false judgment and zero score must not fall back to an AI true result.
for(const ai of [true,false])for(const teacher of [true,false]){
 const task={result:{correct:ai,feedback:'initial'},final_review:{correct:teacher,score:0}}
 assert.equal(effectiveCorrect(task),teacher)
}
assert.equal(effectiveCorrect({result:{correct:false}}),false)
assert.equal(effectiveCorrect({}),undefined)
assert.equal(readSubject('physics'),'science')
// A11-S2: stale all/invalid filters restore a concrete subject.
for(const raw of [null,'','all','other'])assert.equal(readSubject(raw),'math')
for(const subject of ['chinese','math','english','science','morality'])assert.equal(readSubject(subject),subject)
