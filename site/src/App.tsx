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
      <main className="mx-auto max-w-6xl border-x">
        <Hero />
        <Features />
        <GettingStarted />
        <Safety />
        <Download />
        <Faq />
      </main>
      <div className="border-t" />
      <Footer />
    </div>
  )
}
