import assert from 'node:assert/strict'
import { usedCitations } from '../src/citations.ts'
const citations=[{index:1,material_id:1},{index:2,material_id:2}]
assert.deepEqual(usedCitations('尚未引用',citations),[])
assert.deepEqual(usedCitations('有依据[2]，再次引用[2]',citations),[citations[1]])
assert.deepEqual(usedCitations('编号[99]不在证据中',citations),[])
console.log('PASS: retrieved candidates only become visible references when the answer uses their indices')
