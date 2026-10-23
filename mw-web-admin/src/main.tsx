import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { AppToaster } from './components/app-toaster'
import { initializeTheme } from './lib/theme'
import App from './App'
import './styles/globals.css'

initializeTheme()

// After a deploy the hashed chunks of the previous build are gone. A tab that
// still runs the old build fails to import them; reload once to pick up the new
// index.html. The timestamp guard prevents a reload loop if the failure persists.
window.addEventListener('vite:preloadError', (event) => {
  const key = 'mw-admin:chunk-reload'
  try {
    const last = Number(sessionStorage.getItem(key) ?? 0)
    if (Date.now() - last < 30_000) return
    sessionStorage.setItem(key, String(Date.now()))
  } catch {
    return
  }
  event.preventDefault()
  window.location.reload()
})

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

const routerBasename = import.meta.env.BASE_URL.replace(/\/app\/?$/, '') || '/'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter basename={routerBasename}>
        <App />
      </BrowserRouter>
      <AppToaster />
    </QueryClientProvider>
  </StrictMode>,
)
