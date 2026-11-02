import { StrictMode } from 'react'
import { ensureHydroGlobals } from './upstream/hydro-globals.ts'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App.tsx'
import { AuthProvider } from './auth.tsx'
import { SiteSettingsProvider } from './site-settings.tsx'
import './index.css'

ensureHydroGlobals()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <SiteSettingsProvider>
        <AuthProvider>
          <App />
        </AuthProvider>
      </SiteSettingsProvider>
    </BrowserRouter>
  </StrictMode>,
)
