import { useEffect } from 'react'
import { useAtomValue } from 'jotai'
import { langAtom } from './state'
import { htmlLang } from './i18n'
import { Header } from './sections/Header'
import { Changelog } from './sections/Changelog'
import { Footer } from './sections/Footer'

/** /changelog: what changed in each version, from CHANGELOG.md. */
export function ChangelogPage() {
  const lang = useAtomValue(langAtom)
  useEffect(() => {
    document.documentElement.lang = htmlLang(lang)
  }, [lang])
  return (
    <div>
      <Header home={false} />
      <main className="mx-auto max-w-6xl px-4 pt-14 sm:px-6">
        <Changelog />
      </main>
      <Footer />
    </div>
  )
}
