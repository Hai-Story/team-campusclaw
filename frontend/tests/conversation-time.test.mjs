import assert from 'node:assert/strict'
import { absoluteTime,relativeTime,touchConversation } from '../src/conversation-time.ts'
import { applyMessageEvent } from '../src/stream-state.ts'

const stamp='2026-10-23T00:05:00Z',ms=Date.parse(stamp)
assert.equal(absoluteTime(stamp),'2026年10月23日08:05')
assert.equal(absoluteTime('2026-10-23T08:05:00+08:00'),absoluteTime(stamp))
assert.equal(absoluteTime('2026-10-22T16:00:00Z'),'2026年10月23日00:00')
for(const [seconds,expected] of [[0,'刚刚'],[59,'刚刚'],[60,'1分钟前'],[3599,'59分钟前'],[3600,'1小时前'],[86400,'1天前'],[2592000,'1个月前'],[31536000,'1年前'],[-90,'刚刚']])assert.equal(relativeTime(stamp,ms+seconds*1000),expected)
for(const invalid of [undefined,'','oops','2026-10-23 08:05:00']){assert.equal(absoluteTime(invalid),'');assert.equal(relativeTime(invalid,ms),'时间未知')}
const list=[{id:1,updated_at:'2026-10-22T00:00:00Z'},{id:2,updated_at:'2026-10-22T01:00:00Z'}]
const touched=touchConversation(list,1,stamp)
assert.deepEqual(touched.map(c=>c.id),[1,2]);assert.equal(list[0].updated_at,'2026-10-22T00:00:00Z')
assert.equal(touchConversation(touched,1,'2020-01-01T00:00:00Z')[0].updated_at,stamp)
assert.equal(touchConversation(list,1,'bad'),list)
let message={id:-1,role:'assistant',content:'',citations:[],status:'partial',created_at:''}
message=applyMessageEvent(message,{event:'meta',data:{assistant_created_at:stamp}})
message=applyMessageEvent(message,{event:'delta',data:{text:'回答'}})
message=applyMessageEvent(message,{event:'meta',data:{citations:[]}})
assert.equal(message.created_at,stamp)
message=applyMessageEvent(message,{event:'done',data:{message_id:42,created_at:stamp}})
assert.equal(message.created_at,stamp);assert.equal(message.id,42);assert.equal(message.status,'complete')
console.log('PASS: relative boundaries, Beijing midnight/offset, invalid dates, independent activity and stable stream time')
