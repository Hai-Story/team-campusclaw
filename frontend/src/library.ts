import { subjects } from './subjects.ts'
export const categories=[...subjects.map(x=>x.name),'其他'] as const
export function materialCategory(title:string):typeof categories[number]{
 if(/语文|阅读|写作|作文|古诗|春晓|chinese/i.test(title))return subjects[0].name
 if(/数学|方程|分数|几何|math/i.test(title))return subjects[1].name
 if(/英语|english/i.test(title))return subjects[2].name
 if(/物理|磁铁|运动学|physics/i.test(title))return subjects[3].name
 if(/化学|酸碱|chemistry/i.test(title))return subjects[4].name
 if(/生物|光合|biology/i.test(title))return subjects[5].name
 if(/政治|思想品德|品德|道德|法治|思政|morality|ethics|politics/i.test(title))return subjects[6].name
 if(/历史|history/i.test(title))return subjects[7].name
 if(/地理|geography/i.test(title))return subjects[8].name
 if(/科学|science/i.test(title))return subjects[3].name
 return '其他'
}
// Each independent loader owns a gate. Old successes AND errors cannot replace a newer selection.
export function latestRequest(){let version=0;return {begin:()=>++version,current:(id:number)=>id===version,cancel:()=>{version++}}}
