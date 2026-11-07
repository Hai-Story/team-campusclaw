export const subjects = [{id:'chinese',name:'语文'},{id:'math',name:'数学'},{id:'english',name:'英语'},{id:'science',name:'科学'},{id:'morality',name:'思想品德'}] as const
export type Subject = typeof subjects[number]['id']
