import assert from 'node:assert/strict'
import { applyMessageEvent } from '../src/stream-state.ts'
import { addToolReference } from '../src/skill-tools.ts'
const citation={index:1,material_id:7,material_title:'本班资料',chunk_index:0,char_start:0,char_end:5,excerpt:'资料摘要'}
let m={id:-1,role:'assistant',content:'',citations:[],skill_name:'',status:'partial'}
m=applyMessageEvent(m,{event:'meta',data:{citations:[citation],skill:{name:'数学解题引导'}}})
m=applyMessageEvent(m,{event:'meta',data:{tool_calls:[{id:10,ref:'formula_hint@campus-tools',status:'ok',text:'方法提示',round:1}]}})
m=applyMessageEvent(m,{event:'meta',data:{tool_calls:[{id:11,ref:'unit_convert@campus-tools',status:'error',text:'工具失败',round:2}]}})
assert.equal(m.citations[0],citation);assert.equal(m.skill_name,'数学解题引导');assert.equal(m.tool_calls.length,2)
m=applyMessageEvent(m,{event:'meta',data:{tool_calls:[m.tool_calls[0]]}});assert.equal(m.tool_calls.length,2)
m=applyMessageEvent(m,{event:'delta',data:{text:'先看'}});m=applyMessageEvent(m,{event:'delta',data:{text:'单位。'}});assert.equal(m.content,'先看单位。')
m=applyMessageEvent(m,{event:'done',data:{message_id:90}});assert.equal(m.status,'complete');assert.equal(m.id,90);assert.equal(m.tool_calls.length,2)
const plain={id:1,role:'assistant',content:'普通回答',citations:[],skill_name:'',status:'complete'};assert.equal(applyMessageEvent(plain,{event:'meta',data:{}}).tool_calls,undefined)
const source='名称：引导\r\n工具：knowledge_search\r\n启用：是\r\n\r\n内容中 mention 工具：不要改变这行。'
const inserted=addToolReference(source,'formula_hint@campus-tools');assert.ok(inserted.includes('工具：knowledge_search,formula_hint@campus-tools\n'));assert.equal(addToolReference(inserted,'formula_hint@campus-tools'),inserted);assert.ok(inserted.endsWith('内容中 mention 工具：不要改变这行。'))
assert.ok(addToolReference('名称：引导\n启用：是\n\n说明','unit_convert@campus-tools').includes('工具：unit_convert@campus-tools\n\n说明'))
console.log('PASS: incremental MCP events preserve citations/history and safe skill reference insertion')
