type Definition = {name:string;prompt:string}
// Conservative local examples: an unknown role gets no invented capabilities.
const topics = [
 {match:/单位|换算|进率|unit\s*conver/i, examples:['米换成厘米时，应该怎样确定进率？','换算单位前，我应该先检查什么？','请提示我如何检查这一步的单位是否一致。']},
 {match:/数学|方程|分数|几何|代数|math|algebra/i, examples:['去括号时该怎么检查？','分数相加要注意什么？','列方程前怎样找出题目中的数量关系？']},
 {match:/英语|英文|english|grammar|vocabulary/i, examples:['这句话的语法应该怎样分析？','怎样区分这两个英文单词的用法？','请帮我检查这段英文表达，并解释修改原因。']},
 {match:/语文|阅读|古诗|诗词|文言/i, examples:['阅读理解怎样找到作者用意？','分析古诗时，我应该先关注哪些词句？','怎样用文章中的依据说明自己的理解？']},
 {match:/编程|代码|程序设计|python|javascript|coding|programming/i, examples:['这段代码为什么报错，我应该先检查哪里？','怎样把这个需求拆成清楚的实现步骤？','我该怎样验证这段代码是否符合预期？']},
 {match:/科学|物理|化学|生物|实验|science|physics|chemistry|biology/i, examples:['能用生活中的例子解释这个现象吗？','如何设计一个简单实验验证我的猜想？','我应该怎样比较这两个概念的区别？']},
 {match:/写作|作文|文章润色|writing/i, examples:['怎样让这段文字表达得更具体？','我想写好开头，应该先确定什么？','请帮我检查文章结构，并给出改进方向。']},
 {match:/历史|历史事件|history/i, examples:['怎样梳理这个历史事件的背景和影响？','如何比较这两个历史事件的异同？','我应该用哪些史实支持这个判断？']},
 {match:/解题|解答|解惑|答疑|problem.solv|tutor/i, examples:['你能帮我检查这一步的思路吗？','我已经做了一部分，下一步该检查什么？','怎样判断我对题目条件的理解是否正确？']},
 {match:/知识讲解|概念|科普|讲解助手|explain/i, examples:['能用一个简单例子解释这个概念吗？','学习这个知识点前，需要先了解什么？','怎样区分这两个容易混淆的概念？']},
]
export function agentSuggestions(agent?:Definition):string[] {
 if(!agent)return []
 // A function stated in the name outweighs incidental words in a longer prompt.
 const matched=topics.map((topic,order)=>({topic,order,score:(topic.match.test(agent.name)?8:0)+(topic.match.test(agent.prompt)?3:0)})).filter(v=>v.score>0).sort((a,b)=>b.score-a.score||a.order-b.order)
 return matched[0]?.topic.examples??[]
}
