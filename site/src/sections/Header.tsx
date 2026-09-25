import { useAtom } from 'jotai'
import { Globe } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useT } from '@/i18n'
import { langAtom, REPOSITORY } from '@/state'

/** The top bar: mark and name, the section links, GitHub, language, download. */
export function Header({ home = true }: { home?: boolean }) {
  const t = useT()
  const [lang, setLang] = useAtom(langAtom)
  const prefix = home ? '' : '/'
  const links: [string, string][] = [
    [`${prefix}#features`, t.nav.features],
    [`${prefix}#start`, t.nav.start],
    [`${prefix}#safety`, t.nav.safety],
    [`${prefix}#faq`, t.nav.faq],
  ]
  return (
    <header className="bg-background/90 sticky top-0 z-20 border-b backdrop-blur">
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-6 border-x px-6 sm:px-10">
        <a href="/" className="flex items-center gap-2.5">
          <img src="/assets/mayak-mark.png" alt="" width={28} height={28} className="size-7" />
          <span className="text-lg font-extrabold tracking-tight">MAYAK</span>
        </a>
        <nav className="hidden items-center gap-6 text-sm md:flex">
          {links.map(([href, label]) => (
            <a key={href} href={href} className="text-muted-foreground hover:text-foreground transition-colors">
              {label}
            </a>
          ))}
          <a href={`https://github.com/${REPOSITORY}`} rel="noopener" className="text-muted-foreground hover:text-foreground transition-colors">
            {t.nav.github}
          </a>
        </nav>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="icon" onClick={() => setLang(lang === 'ja' ? 'en' : 'ja')} aria-label={t.nav.lang} title={t.nav.lang}>
            <Globe className="size-5" />
          </Button>
          <Button asChild className="font-semibold">
            <a href={`${prefix}#download`}>{t.nav.downloadButton}</a>
          </Button>
        </div>
      </div>
    </header>
  )
}
