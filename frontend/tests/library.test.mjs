import assert from 'node:assert/strict'
import { materialCategory,latestRequest } from '../src/library.ts'
assert.equal(materialCategory('A班–小学科学课程资料.md'),'科学')
assert.equal(materialCategory('语文自学笔记.md'),'语文')
assert.equal(materialCategory('方程与分数.txt'),'数学')
assert.equal(materialCategory('English reading.md'),'英语')
assert.equal(materialCategory('劳动教育.md'),'劳动')
assert.equal(materialCategory('作业5测试讲义.md'),'其他')
const gate=latestRequest(),list=latestRequest(),old=gate.begin(),current=gate.begin()
assert.equal(gate.current(old),false)
assert.equal(gate.current(current),true)
list.begin();assert.equal(gate.current(current),true)
gate.cancel();assert.equal(gate.current(current),false)
const values=new Map([['campusclaw_token','local-test-token']]);globalThis.localStorage={getItem:k=>values.get(k)??null,removeItem:k=>values.delete(k)}
const {materialFile,setUnauthorizedHandler}=await import('../src/api.ts')
let expired=0;setUnauthorizedHandler(()=>expired++)
globalThis.fetch=async(path,init)=>{assert.equal(path,'/api/materials/12/file');assert.equal(init.headers.Authorization,'Bearer local-test-token');return new Response('课件原文',{status:200})}
assert.equal(await (await materialFile(12)).text(),'课件原文')
globalThis.fetch=async()=>new Response(JSON.stringify({error:'未登录'}),{status:401})
await assert.rejects(materialFile(12),/未登录/);assert.equal(expired,1);assert.equal(values.has('campusclaw_token'),false)
console.log('PASS: material classification, stale selection/unmount protection and authenticated downloads')
