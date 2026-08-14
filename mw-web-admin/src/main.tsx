import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { AppToaster } from './components/app-toaster'
import { initializeTheme } from './lib/theme'
import App from './App'
import './styles/globals.css'

initializeTheme()

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
