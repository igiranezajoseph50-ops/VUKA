import React from 'react'
import ReactDOM from 'react-dom/client'
import { HashRouter } from 'react-router-dom'
import App from './App'
import { TraderProvider } from './state/TraderContext'
import { ToastProvider } from './components/ui/ToastProvider'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <HashRouter>
      <ToastProvider>
        <TraderProvider>
          <App />
        </TraderProvider>
      </ToastProvider>
    </HashRouter>
  </React.StrictMode>,
)
