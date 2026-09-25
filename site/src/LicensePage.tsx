import { useEffect } from 'react'
import { useAtomValue } from 'jotai'
import { langAtom } from './state'
import { htmlLang } from './i18n'
import { Header } from './sections/Header'
import { Licenses } from './sections/Licenses'
import { Footer } from './sections/Footer'

/** /license: the licenses and credits, off the front page. */
export function LicensePage() {
  const lang = useAtomValue(langAtom)
  useEffect(() => {
    document.documentElement.lang = htmlLang(lang)
  }, [lang])
  return (
    <div>
      <Header home={false} />
      <main className="mx-auto max-w-6xl px-4 pt-14 sm:px-6">
        <Licenses />
      </main>
      <Footer />
    </div>
  )
}
