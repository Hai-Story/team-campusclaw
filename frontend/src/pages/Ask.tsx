import { useRef, useState, type FormEvent } from 'react'
import {
  api,
  type Citation,
  type Me,
  type SearchHit,
  type SearchMode,
} from '../api'
import { useToast } from '../components/Toast'
import type { Theme } from '../App'

interface Props {
  me: Me
  theme: Theme
  onToggleTheme: () => void
  onLogout: () => void
  onOpenMaterials: () => void
  onOpenMaterial: (id: number) => void
}

interface ChatItem {
  role: 'user' | 'assistant'
  content: string
  citations?: Citation[]
}

const MODE_LABELS: Record<SearchMode, string> = {
  keyword: '关键字',
  vector: '向量',
  hybrid: '混合（默认）',
}

export default function AskPage({ me, onOpenMaterial }: Props) {
  const [tab, setTab] = useState<'search' | 'qa'>('qa')
  const toast = useToast()

  // --- 知识检索 ---
  const [query, setQuery] = useState('')
  const [mode, setMode] = useState<SearchMode>('hybrid')
  const [hits, setHits] = useState<SearchHit[] | null>(null)
  const [searching, setSearching] = useState(false)

  // --- 知识问答 ---
  const [messages, setMessages] = useState<ChatItem[]>([])
  const [question, setQuestion] = useState('')
  const [asking, setAsking] = useState(false)
  const chatEnd = useRef<HTMLDivElement>(null)

  async function doSearch(e: FormEvent) {
    e.preventDefault()
    if (!query.trim()) return
    setSearching(true)
    try {
      const res = await api.search(query.trim(), mode)
      setHits(res.hits ?? [])
    } catch (err) {
      if (err instanceof Error && err.message !== '未登录') {
        toast(err.message, true)
        setHits([])
      }
    } finally {
      setSearching(false)
    }
  }

  async function doAsk(e: FormEvent) {
    e.preventDefault()
    const q = question.trim()
    if (!q) return
    setAsking(true)
    setQuestion('')
    setMessages((m) => [...m, { role: 'user', content: q }])
    try {
      // 历史仅附带（不参与检索）；客户端 system 注入由服务端丢弃
      const history = messages.map((m) => ({ role: m.role, content: m.content }))
      const res = await api.ask(q, history)
      setMessages((m) => [...m, { role: 'assistant', content: res.answer, citations: res.citations }])
      setTimeout(() => chatEnd.current?.scrollIntoView({ behavior: 'smooth' }), 50)
    } catch (err) {
      const msg = err instanceof Error && err.message !== '未登录' ? err.message : '问答失败'
      setMessages((m) => [...m, { role: 'assistant', content: msg, citations: [] }])
    } finally {
      setAsking(false)
    }
  }

  return (
    <>


      <main className="container">
        <h1 className="ask-title">{me.class_name} · 知识问答</h1>
        <p className="ask-sub">
          {me.username} · {me.role === 'teacher' ? '教师' : '学生'}
        </p>

        <div className="ask-tabs">
          <button className={tab === 'search' ? 'tab active' : 'tab'} onClick={() => setTab('search')}>
            知识检索
          </button>
          <button className={tab === 'qa' ? 'tab active' : 'tab'} onClick={() => setTab('qa')}>
            知识问答
          </button>
        </div>

        {tab === 'search' && (
          <>
            <p className="hint">
              在本班材料切片中检索：关键字模式只看词是否出现；向量模式看语义是否接近（低于配置阈值
              的候选被丢弃）；混合模式为两路按名次 RRF 融合（默认）。
            </p>
            <form className="toolbar" onSubmit={doSearch}>
              <input
                className="search"
                type="text"
                placeholder="输入一句自然语言或关键词…"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
              <select value={mode} onChange={(e) => setMode(e.target.value as SearchMode)}>
                {(['hybrid', 'keyword', 'vector'] as SearchMode[]).map((m) => (
                  <option key={m} value={m}>
                    {MODE_LABELS[m]}
                  </option>
                ))}
              </select>
              <button className="primary" type="submit" disabled={searching}>
                {searching ? '检索中…' : '检索'}
              </button>
            </form>

            {hits !== null && hits.length === 0 && <div className="empty">资料中未找到相关内容</div>}
            {hits !== null && hits.length > 0 && (
              <div className="hit-list">
                {hits.map((h) => (
                  <div key={h.chunk_id} className="hit-item" onClick={() => onOpenMaterial(h.material_id)}>
                    <div className="hit-head">
                      《{h.material_title}》 切片{h.chunk_index} · 偏移{h.char_start}–{h.char_end}
                      <span className="hit-scores">
                        {h.rrf_score !== undefined && `RRF ${h.rrf_score.toFixed(4)}`}
                        {h.keyword_score !== undefined && ` · 全文 ${h.keyword_score.toFixed(2)}`}
                        {h.vector_score !== undefined && ` · 相似度 ${h.vector_score.toFixed(3)}`}
                      </span>
                    </div>
                    <div className="hit-excerpt">{h.excerpt}</div>
                  </div>
                ))}
              </div>
            )}
          </>
        )}

        {tab === 'qa' && (
          <>
            <p className="hint">
              每个问题先在本班做混合检索，取前 4 条切片再生成简短回答，并标出
              [1]、[2]。没有命中时不调用模型，直接说明资料里没有。
            </p>
            <div className="chat-list">
              {messages.map((m, i) =>
                m.role === 'user' ? (
                  <div key={i} className="bubble-user">
                    <div className="bubble-label">你</div>
                    {m.content}
                  </div>
                ) : (
                  <div key={i} className="answer-card">
                    <div className="answer-label">依据回答</div>
                    <div className="answer-text">{m.content}</div>
                    {m.citations && m.citations.length > 0 && (
                      <ol className="citation-list">
                        {m.citations.map((c) => (
                          <li key={c.index}>
                            <button
                              className="citation-link"
                              onClick={() => onOpenMaterial(c.material_id)}
                            >
                              [{c.index}] {c.material_title} 切片{c.chunk_index} · 偏移{c.char_start}–{c.char_end}
                            </button>
                            <div className="hit-excerpt">{c.excerpt}</div>
                          </li>
                        ))}
                      </ol>
                    )}
                  </div>
                ),
              )}
              <div ref={chatEnd} />
            </div>
            <form className="toolbar ask-input" onSubmit={doAsk}>
              <input
                className="search"
                type="text"
                placeholder="向本班知识库提问…"
                value={question}
                onChange={(e) => setQuestion(e.target.value)}
              />
              <button className="primary" type="submit" disabled={asking}>
                {asking ? '生成中…' : '发送'}
              </button>
            </form>
          </>
        )}
      </main>
    </>
  )
}
