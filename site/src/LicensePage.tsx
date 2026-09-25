import { useEffect } from 'react'
import { useAtomValue } from 'jotai'
import { langAtom } from './state'
import { Header } from './sections/Header'
import { Licenses } from './sections/Licenses'
import { Footer } from './sections/Footer'

/** /license: the licenses and credits, off the front page. */
export function LicensePage() {
  const lang = useAtomValue(langAtom)
  useEffect(() => {
    document.documentElement.lang = lang
  }, [lang])
  return (
    <div>
      <Header home={false} />
      <main className="mx-auto max-w-6xl border-x">
        <Licenses />
      </main>
      <div className="border-t" />
      <Footer />
    </div>
  )
}
