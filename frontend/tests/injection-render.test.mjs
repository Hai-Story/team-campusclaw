import assert from 'node:assert/strict'
import {build} from '../node_modules/esbuild/lib/main.js'
import {mkdir,rm} from 'node:fs/promises'
import {pathToFileURL} from 'node:url'
const dir=new URL('../node_modules/.cache/security-render/',import.meta.url)
await mkdir(dir,{recursive:true})
try{
 const file=new URL('titles.mjs',dir)
 await build({entryPoints:[new URL('../src/components/MaterialTitle.tsx',import.meta.url).pathname],outfile:file.pathname,bundle:true,platform:'node',format:'esm',jsx:'automatic',packages:'external'})
 const {default:Title}=await import(pathToFileURL(file.pathname).href)
 const {createElement}=await import('../node_modules/react/index.js'),{renderToStaticMarkup}=await import('../node_modules/react-dom/server.node.js')
 for(const title of ['<script>alert(1)</script>','<img src=x onerror=alert(1)>','</span><svg onload=alert(1)>']){
  const markup=renderToStaticMarkup(createElement(Title,{title}))
  assert.ok(markup.includes('&lt;'));assert.ok(!/<(?:script|img|svg)\b/i.test(markup));assert.ok(!/\son(?:error|load)=/i.test(markup.replace(/&lt;.*?&gt;/g,'')))
 }
 console.log('PASS S3 XSS: actual material title component escapes stored script/img/svg inputs; no executable tags or handlers')
}finally{await rm(dir,{recursive:true,force:true})}
