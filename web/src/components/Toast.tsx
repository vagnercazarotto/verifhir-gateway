import { createContext, useCallback, useContext, useRef, useState } from 'react'

export type ToastKind = 'success' | 'error' | 'info'

interface ToastItem {
  id: number
  message: string
  kind: ToastKind
}

interface ToastCtx {
  toast: (message: string, kind?: ToastKind) => void
}

const Ctx = createContext<ToastCtx>({ toast: () => {} })

export const useToast = () => useContext(Ctx)

const KIND_CLS: Record<ToastKind, string> = {
  success: 'bg-emerald-500',
  error:   'bg-red-500',
  info:    'bg-brand-500',
}

const KIND_ICON: Record<ToastKind, string> = {
  success: '✓',
  error:   '✕',
  info:    'ℹ',
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])
  const counter = useRef(0)

  const toast = useCallback((message: string, kind: ToastKind = 'success') => {
    const id = ++counter.current
    setItems(prev => [...prev, { id, message, kind }])
    setTimeout(() => setItems(prev => prev.filter(t => t.id !== id)), 3500)
  }, [])

  return (
    <Ctx.Provider value={{ toast }}>
      {children}
      <div className="fixed bottom-5 right-5 z-50 flex flex-col gap-2 pointer-events-none">
        {items.map(t => (
          <div
            key={t.id}
            className={`${KIND_CLS[t.kind]} text-white text-sm px-4 py-2.5 rounded-lg shadow-lg pointer-events-auto flex items-center gap-2 animate-toast`}
          >
            <span className="font-bold">{KIND_ICON[t.kind]}</span>
            {t.message}
          </div>
        ))}
      </div>
    </Ctx.Provider>
  )
}
