// API 封装：同源 /api；凭据为登录下发的不透明 Token——
// 存 localStorage（键 campusclaw_token，作业图示要求），请求经 Authorization: Bearer 头携带；
// 任何 401 都会清除本地 Token 并回登录页（spec R1）。
// 前端不存储角色（localStorage 只有 token），身份一律以 GET /api/me 为准。

export interface Me {
  id: number
  username: string
  role: 'teacher' | 'student'
  class_id: number
  class_name: string
}

export interface MaterialSummary {
  id: number
  title: string
  class_name: string
  created_at: string
}

export interface MaterialDetail extends MaterialSummary {
  version: number
  body: string
}

// ===== 知识库检索与问答 =====

/** 一条可溯源的检索命中（材料标题、切片序号、字符区间、摘录齐备）。 */
export interface SearchHit {
  material_id: number
  material_title: string
  chunk_id: number
  chunk_index: number
  char_start: number
  char_end: number
  excerpt: string
  keyword_score?: number
  vector_score?: number
  rrf_score?: number
}

/** 问答引用出处，[n] 与 citations 顺序一致。 */
export interface Citation {
  index: number
  material_id: number
  material_title: string
  chunk_index: number
  char_start: number
  char_end: number
  excerpt: string
}

export interface AskResponse {
  answer: string
  citations: Citation[]
}

export type SearchMode = 'keyword' | 'vector' | 'hybrid'

// localStorage 中仅存不透明 Token，不存角色/班级等授权判断依据（spec R2/R9）。
const TOKEN_KEY = 'campusclaw_token'

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? ''
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

function authHeaders(): Record<string, string> {
  const token = getToken()
  return token ? { Authorization: `Bearer ${token}` } : {}
}

let onUnauthorized: () => void = () => {}
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

// 统一的 401 处理：清除本地 Token 并回登录页（spec R1「前端收到 401 回登录页」）。
function handleUnauthorized() {
  clearToken()
  onUnauthorized()
}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, { ...init, headers: { ...authHeaders(), ...init?.headers } })
  if (res.status === 401 && path !== '/api/login') {
    handleUnauthorized()
    throw new Error('未登录')
  }
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const message = path === '/api/login' && res.status === 401
      ? '账号或密码错误'
      : (data as { error?: string }).error ?? `请求失败（${res.status}）`
    const err = new Error(message) as Error & { status?: number }
    err.status = res.status
    throw err
  }
  return data as T
}

export const api = {
  updateMaterial:(id:number,title:string,body:string,version:number)=>request<{id:number;title:string;version:number;index_status:string}>(`/api/materials/${id}`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({title,body,version})}),
  deleteMaterial:(id:number,version:number)=>request<void>(`/api/materials/${id}`,{method:'DELETE',headers:{'Content-Type':'application/json'},body:JSON.stringify({version})}),
  login: async (username: string, password: string): Promise<Me> => {
    const data = await request<{ token: string; user: Me }>('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
    setToken(data.token) // 登录成功：Token 写入 localStorage（spec R1「Token 存储于 localStorage」）
    return data.user
  },
  logout: async () => {
    try {
      await request<{ ok: boolean }>('/api/logout', { method: 'POST' })
    } finally {
      clearToken() // 服务端删行 + 本地清除，无论响应成败都退出登录态（design.md D6）
    }
  },
  me: () => request<Me>('/api/me'),
  listMaterials: (q: string) =>
    request<MaterialSummary[]>(`/api/materials${q ? `?q=${encodeURIComponent(q)}` : ''}`),
  materialDetail: (id: number) => request<MaterialDetail>(`/api/materials/${id}`),
  // 检索与问答均只指向本站 /api（前端绝不直连 Qdrant 或网关）
  search: (query: string, mode: SearchMode) =>
    request<{ mode: SearchMode; hits: SearchHit[] }>('/api/search', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query, mode }),
    }),
  ask: (question: string, history: { role: string; content: string }[]) =>
    request<AskResponse>('/api/ask', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ question, history }),
    }),
}

// 上传走 XHR 以获得进度事件（spec「界面体验」上传进度）；同样携带 Bearer Token。
export function uploadMaterial(
  file: File,
  title: string,
  onProgress: (percent: number) => void,
): Promise<{ id: number; title: string }> {
  return new Promise((resolve, reject) => {
    const form = new FormData()
    form.append('file', file)
    if (title) form.append('title', title)
    const xhr = new XMLHttpRequest()
    xhr.open('POST', '/api/materials')
    const token = getToken()
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100))
    }
    xhr.onload = () => {
      if (xhr.status === 401) {
        handleUnauthorized()
        reject(new Error('未登录'))
        return
      }
      let data: { id?: number; title?: string; error?: string } = {}
      try {
        data = JSON.parse(xhr.responseText)
      } catch {
        /* 忽略非 JSON 响应 */
      }
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve({ id: data.id ?? 0, title: data.title ?? file.name })
      } else {
        const err = new Error(data.error ?? `上传失败（${xhr.status}）`) as Error & { status?: number }
        err.status = xhr.status
        reject(err)
      }
    }
    xhr.onerror = () => reject(new Error('网络错误'))
    xhr.send(form)
  })
}

// File requests must carry the same Bearer credentials as detail requests.
export async function materialFile(id:number):Promise<Blob>{
 const res=await fetch(`/api/materials/${id}/file`,{headers:authHeaders()})
 if(res.status===401){handleUnauthorized();throw new Error('未登录')}
 if(!res.ok){const data=await res.json().catch(()=>({}));throw new Error(data.error??`下载失败（${res.status}）`)}
 return res.blob()
}
export async function downloadMaterial(id:number,title:string){
 const blob=await materialFile(id),url=URL.createObjectURL(blob),link=document.createElement('a')
 link.href=url;link.download=title.replace(/[\\/:*?"<>|]/g,'_');document.body.append(link);link.click();link.remove();setTimeout(()=>URL.revokeObjectURL(url),1000)
}
