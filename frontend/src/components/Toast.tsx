import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from 'react'

interface ToastItem {
  id: number
  text: string
  error: boolean
}

const ToastCtx = createContext<(text: string, error?: boolean) => void>(() => {})

export function useToast() {
  return useContext(ToastCtx)
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])
  const seq = useRef(0)

  const push = useCallback((text: string, error = false) => {
    const id = ++seq.current
    setItems((list) => [...list, { id, text, error }])
    setTimeout(() => setItems((list) => list.filter((t) => t.id !== id)), 3200)
  }, [])

  return (
    <ToastCtx.Provider value={push}>
      {children}
      <div className="toast-wrap">
        {items.map((t) => (
          <div key={t.id} className={t.error ? 'toast error' : 'toast'}>
            {t.text}
          </div>
        ))}
      </div>
    </ToastCtx.Provider>
  )
}
