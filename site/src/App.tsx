import { useEffect } from 'react'
import { useAtom } from 'jotai'
import { langAtom } from './state'
import { Header } from './sections/Header'
import { Hero } from './sections/Hero'
import { Features } from './sections/Features'
import { GettingStarted } from './sections/GettingStarted'
import { Safety } from './sections/Safety'
import { Download } from './sections/Download'
import { Faq } from './sections/Faq'
import { Footer } from './sections/Footer'

export function App() {
  const [lang] = useAtom(langAtom)
  useEffect(() => {
    document.documentElement.lang = lang
  }, [lang])
  return (
    <div>
      <Header />
      <Hero />
      <main className="mx-auto max-w-6xl px-4 sm:px-6">
        <Features />
        <GettingStarted />
        <Safety />
        <Download />
        <Faq />
      </main>
      <Footer />
    </div>
  )
}
