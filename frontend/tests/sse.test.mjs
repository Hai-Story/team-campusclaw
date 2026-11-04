import { SSEParser } from '../src/sse.ts'
import assert from 'node:assert/strict'
const data='event: meta\r\ndata: {"citations":[]}\r\n\r\nevent: delta\ndata: {"text":"中文增量"}\n\nevent: done\ndata: {"message_id":7}\n\n'
for(let size=1;size<=20;size++){
 const parser=new SSEParser(),decoder=new TextDecoder(),bytes=new TextEncoder().encode(data),events=[]
 for(let i=0;i<bytes.length;i+=size)parser.push(decoder.decode(bytes.slice(i,i+size),{stream:true}),v=>events.push(v))
 parser.push(decoder.decode(),v=>events.push(v))
 assert.deepEqual(events.map(v=>v.event),['meta','delta','done'])
 assert.equal(events[1].data.text,'中文增量');assert.equal(events[2].data.message_id,7)
}
console.log('PASS: split UTF-8, CRLF, frame boundaries and terminal events')
