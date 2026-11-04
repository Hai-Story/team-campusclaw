import assert from 'node:assert/strict'
import { addToolReference,setSkillEnabled } from '../src/skill-tools.ts'
const source='---\nname: 指导\ndescription: 引导\ntools: [knowledge_search]\nenabled: true\n---\n\n正文中 enabled: true 必须保留。'
const disabled=setSkillEnabled(source,false)
assert.ok(disabled.includes('enabled: false\n---'))
assert.ok(disabled.endsWith('正文中 enabled: true 必须保留。'))
const inserted=addToolReference(disabled,'formula_hint@campus-tools')
assert.ok(inserted.includes('tools: [knowledge_search, formula_hint@campus-tools]\n'))
assert.equal(addToolReference(inserted,'formula_hint@campus-tools'),inserted)
assert.ok(setSkillEnabled('---\nname: x\ndescription: y\n---\n\nbody',false).includes('enabled: false\n---'))
assert.equal(setSkillEnabled('名称：指导\n启用：是\n\n正文',false),'名称：指导\n启用：否\n\n正文')
console.log('PASS: standard and legacy skill toggles and tool insertion preserve instructions')
