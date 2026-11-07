import assert from 'node:assert/strict'
import { agentSuggestions } from '../src/agent-suggestions.ts'
import { restoreIfPresent } from '../src/conversation-lifecycle.ts'
const math=agentSuggestions({name:'数学解题助手',prompt:'带学生检查思路'})
const english=agentSuggestions({name:'英语学习助手',prompt:'解释语法'})
const code=agentSuggestions({name:'编程助手',prompt:'帮助学习Python'})
assert.ok(math.length>0&&english.length>0&&code.length>0)
assert.ok(math.some(q=>q.includes('方程')))
assert.ok(english.some(q=>q.includes('英文')))
assert.ok(code.some(q=>q.includes('代码')))
assert.ok(!english.some(q=>q.includes('分数')||q.includes('去括号')))
assert.deepEqual(agentSuggestions(),[])
assert.deepEqual(agentSuggestions({name:'班级助手',prompt:'你好'}),[])
assert.ok(agentSuggestions({name:'问问看',prompt:'你负责单位换算，只给方法'}).some(q=>q.includes('进率')))
// The named role wins over an incidental subject in a longer definition.
assert.deepEqual(agentSuggestions({name:'英语助手',prompt:'必要时使用数学中的例子解释英文语法'}),english)
const deleted=new Set(),restored=[]
let finish
const pending=restoreIfPresent(7,()=>new Promise(resolve=>{finish=resolve}),id=>deleted.has(id),v=>restored.push(v))
deleted.add(7);finish({id:7,messages:['迟到的内容']})
assert.equal(await pending,false);assert.deepEqual(restored,[])
let fetched=false
assert.equal(await restoreIfPresent(7,async()=>{fetched=true;return{}},id=>deleted.has(id),v=>restored.push(v)),false)
assert.equal(fetched,false)
assert.equal(await restoreIfPresent(8,async()=>({id:8}),id=>deleted.has(id),v=>restored.push(v)),true)
assert.deepEqual(restored,[{id:8}])
console.log('PASS: examples follow agent function; unknown roles stay empty; deleted conversations cannot be restored by late responses')
