import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Provider } from 'jotai'
import { ChangelogPage } from './ChangelogPage'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Provider>
      <ChangelogPage />
    </Provider>
  </StrictMode>,
)
