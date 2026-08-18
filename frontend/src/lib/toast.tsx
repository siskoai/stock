// Notifications éphémères. Les messages d'erreur viennent du backend et sont
// déjà rédigés en français : on les affiche tels quels.

import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'

type Tone = 'success' | 'error' | 'info'
interface Toast { id: number; tone: Tone; text: string }

interface ToastValue {
  success: (text: string) => void
  error: (text: string) => void
  info: (text: string) => void
}

const Ctx = createContext<ToastValue | null>(null)
let nextId = 1

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Toast[]>([])

  const dismiss = useCallback((id: number) => {
    setItems((list) => list.filter((t) => t.id !== id))
  }, [])

  const push = useCallback((tone: Tone, text: string) => {
    const id = nextId++
    setItems((list) => [...list, { id, tone, text }])
    // Une erreur reste plus longtemps : elle demande souvent une action.
    window.setTimeout(() => dismiss(id), tone === 'error' ? 7000 : 3500)
  }, [dismiss])

  const value = useMemo<ToastValue>(() => ({
    success: (t) => push('success', t),
    error: (t) => push('error', t),
    info: (t) => push('info', t),
  }), [push])

  return (
    <Ctx.Provider value={value}>
      {children}
      <div className="toasts">
        {items.map((t) => (
          <div key={t.id} className={`toast ${t.tone}`} role="status">
            <span className="toast-text">{t.text}</span>
            <button className="toast-close" onClick={() => dismiss(t.id)} aria-label="Fermer">×</button>
          </div>
        ))}
      </div>
    </Ctx.Provider>
  )
}

export function useToast(): ToastValue {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useToast doit être utilisé dans un ToastProvider')
  return ctx
}
