import { subjects } from './subjects.ts'
export const categories=[...subjects.map(x=>x.name),'其他'] as const
export function materialCategory(title:string):typeof categories[number]{
 if(/语文|阅读|作文|古诗|春晓|chinese/i.test(title))return '语文'
 if(/数学|方程|分数|几何|math/i.test(title))return '数学'
 if(/英语|english/i.test(title))return '英语'
 if(/科学|物理|化学|生物|磁铁|science/i.test(title))return '科学'
 if(/思想品德|品德|道德|法治|思政|政治|morality|ethics/i.test(title))return '思想品德'
 return '其他'
}
// Each independent loader owns a gate. Old successes AND errors cannot replace a newer selection.
export function latestRequest(){let version=0;return {begin:()=>++version,current:(id:number)=>id===version,cancel:()=>{version++}}}
