import assert from 'node:assert/strict'
import {availableBuild} from '../src/build-update.ts'
const current='0123456789abcdef',next='fedcba9876543210',c=new AbortController()
const answer=(value,status=200)=>async(path,init)=>{assert.equal(path,'/build-info.json');assert.equal(init.cache,'no-store');assert.equal(init.credentials,'omit');assert.equal(init.signal,c.signal);assert.equal(init.headers,undefined);return new Response(JSON.stringify(value),{status})}
assert.equal(await availableBuild(current,c.signal,answer({build:current})),null)
assert.equal(await availableBuild(current,c.signal,answer({build:next})),next)
for(const v of [null,{},[],{build:'<script>'},{build:15}])assert.equal(await availableBuild(current,c.signal,answer(v)),null)
assert.equal(await availableBuild(current,c.signal,answer({},503)),null)
await assert.rejects(availableBuild(current,c.signal,async()=>{throw new Error('offline')}),/offline/)
c.abort();await assert.rejects(availableBuild(current,c.signal,async(_,init)=>{init.signal.throwIfAborted()}),{name:'AbortError'})
console.log('PASS U1: new/same/malformed/error/cancel version probes; no-store and no auth headers')
