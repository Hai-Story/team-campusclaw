import assert from 'node:assert/strict'
import { api, request, setUnauthorizedHandler, setToken, getToken } from '../src/api.ts'
const values=new Map();globalThis.localStorage={getItem:k=>values.get(k)??null,setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)}
let expired=0;setUnauthorizedHandler(()=>expired++);setToken('existing-test-token')
globalThis.fetch=async()=>new Response(JSON.stringify({error:'用户名或密码错误'}),{status:401})
await assert.rejects(api.login('wrong','wrong'),/账号或密码错误/);assert.equal(expired,0);assert.equal(getToken(),'existing-test-token')
await assert.rejects(request('/api/me'),/未登录/);assert.equal(expired,1);assert.equal(getToken(),'')
console.log('PASS: login 401 shows credential error; protected 401 clears session')
