export function setSkillEnabled(source:string,enabled:boolean):string{
 const value=source.replace(/\r\n/g,'\n')
 if(value.trimStart().startsWith('---\n')){
  const end=value.indexOf('\n---',value.indexOf('---')+3),head=value.slice(0,end),body=value.slice(end)
  if(end<0)return source
  return (/^enabled\s*:/m.test(head)?head.replace(/^enabled\s*:.*$/m,'enabled: '+enabled):head+'\nenabled: '+enabled)+body
 }
 return value.replace(/^[ \t]*启用[：:][ \t]*(是|否)[ \t]*$/m,'启用：'+(enabled?'是':'否'))
}
export function addToolReference(source:string,ref:string):string {
 source=source.replace(/\r\n/g,'\n')
 if(source.trimStart().startsWith('---\n')){
  const end=source.indexOf('\n---',source.indexOf('---')+3);if(end<0)return source
  const head=source.slice(0,end),body=source.slice(end),line=/^tools\s*:\s*(.*)$/m,match=line.exec(head)
  const refs=match?match[1].replace(/[\[\]"']/g,'').split(/[,，、;；|]/).map(v=>v.trim()).filter(Boolean):[]
  if(!refs.includes(ref))refs.push(ref)
  const replacement='tools: ['+refs.join(', ')+']'
  return (match?head.replace(line,replacement):head+'\n'+replacement)+body
 }
 const split=source.indexOf('\n\n'),head=split<0?source:source.slice(0,split),body=split<0?'':source.slice(split)
 const line=/^[ \t]*工具[ \t]*[：:][ \t]*(.*)$/m,match=line.exec(head)
 if(match){const refs=match[1].split(/[,，、;；|]/).map(v=>v.trim()).filter(Boolean);if(!refs.includes(ref))refs.push(ref);return head.replace(line,'工具：'+refs.join(','))+body}
 return head.trimEnd()+'\n工具：'+ref+body
}
