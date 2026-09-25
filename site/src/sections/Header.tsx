import { useAtom } from 'jotai'
import { ExternalLink, Languages } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useT } from '@/i18n'
import { langAtom, REPOSITORY } from '@/state'

export function Header() {
  const t = useT()
  const [lang, setLang] = useAtom(langAtom)
  const links: [string, string][] = [
    ['#features', t.nav.features],
    ['#start', t.nav.start],
    ['#safety', t.nav.safety],
    ['#download', t.nav.download],
    ['#faq', t.nav.faq],
    ['#license', t.nav.license],
  ]
  return (
    <header className="bg-background/80 sticky top-0 z-20 border-b backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between gap-4 px-4 sm:px-6">
        <a href="#" className="flex items-center gap-2.5">
          <img src="/assets/mayak-logo-white.png" alt="" width={36} height={36} className="size-9" />
          <span className="font-display text-primary text-lg tracking-[0.2em]">MAYAK</span>
        </a>
        <nav className="hidden items-center gap-5 text-sm md:flex">
          {links.map(([href, label]) => (
            <a key={href} href={href} className="text-muted-foreground hover:text-foreground transition-colors">
              {label}
            </a>
          ))}
          <a
            href={`https://github.com/${REPOSITORY}`}
            rel="noopener"
            className="text-muted-foreground hover:text-foreground inline-flex items-center gap-1 transition-colors"
          >
            {t.nav.github}
            <ExternalLink className="size-3.5" />
          </a>
        </nav>
        <Button variant="outline" size="sm" onClick={() => setLang(lang === 'ja' ? 'en' : 'ja')} aria-label="Switch language">
          <Languages />
          {t.nav.lang}
        </Button>
      </div>
    </header>
  )
}
