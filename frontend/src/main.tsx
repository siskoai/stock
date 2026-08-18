import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { SessionProvider } from './lib/session'
import { ToastProvider } from './lib/toast'
import './styles/app.css'

const root = document.getElementById('root')
if (!root) throw new Error('élément racine introuvable')

createRoot(root).render(
  <StrictMode>
    <ToastProvider>
      <SessionProvider>
        <App />
      </SessionProvider>
    </ToastProvider>
  </StrictMode>,
)
