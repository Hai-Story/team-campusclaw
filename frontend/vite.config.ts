import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import {createHash} from 'node:crypto'
import {readFileSync,readdirSync} from 'node:fs'
import {fileURLToPath} from 'node:url'
import {join,relative} from 'node:path'

const root=fileURLToPath(new URL('.',import.meta.url))
function sourceFiles(dir:string):string[]{return readdirSync(dir,{withFileTypes:true}).flatMap(e=>e.isDirectory()?sourceFiles(join(dir,e.name)):[join(dir,e.name)])}
const fingerprint=createHash('sha256')
for(const path of [...sourceFiles(join(root,'src')),...['index.html','package-lock.json','vite.config.ts'].map(p=>join(root,p))].sort())fingerprint.update(relative(root,path)).update(readFileSync(path))
const build=fingerprint.digest('hex').slice(0,16)

// 开发时代理路径与生产 Nginx 保持一致：同源 /api（design.md D1）。
export default defineConfig({
  define:{__APP_BUILD_ID__:JSON.stringify(build)},
  plugins: [react(),{name:'campusclaw-build-info',generateBundle(){this.emitFile({type:'asset',fileName:'build-info.json',source:JSON.stringify({build})})}}],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8081', changeOrigin: false },
      '/health': { target: 'http://localhost:8081', changeOrigin: false },
    },
  },
})
