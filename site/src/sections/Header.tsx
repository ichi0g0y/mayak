import { useEffect, useState } from 'react'
import { useAtom } from 'jotai'
import { ExternalLink, Globe } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useT } from '@/i18n'
import { langAtom, REPOSITORY } from '@/state'
import { cn } from '@/lib/utils'

/** The top bar. On the home page it is transparent over the hero and
 *  turns solid once the page scrolls; on other pages it is always solid. */
export function Header({ home = true }: { home?: boolean }) {
  const t = useT()
  const [lang, setLang] = useAtom(langAtom)
  const [scrolled, setScrolled] = useState(!home)
  useEffect(() => {
    if (!home) return
    const onScroll = () => setScrolled(window.scrollY > 40)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [home])
  const prefix = home ? '' : '/'
  const links: [string, string][] = [
    [`${prefix}#features`, t.nav.features],
    [`${prefix}#start`, t.nav.start],
    [`${prefix}#safety`, t.nav.safety],
    [`${prefix}#download`, t.nav.download],
    [`${prefix}#faq`, t.nav.faq],
  ]
  return (
    <header className={cn('fixed inset-x-0 top-0 z-20 transition-colors duration-300', scrolled ? 'bg-background/85 border-b backdrop-blur' : 'border-b border-transparent')}>
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between gap-4 px-4 sm:px-6">
        <a href="/" className={cn('flex items-center gap-2.5 transition-opacity duration-300', scrolled ? 'opacity-100' : 'opacity-0')} aria-hidden={!scrolled}>
          <img src="/assets/mayak-mark.png" alt="" width={32} height={32} className="size-8" />
          <span className="font-label text-primary text-base">MAYAK</span>
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
        <Button variant="ghost" size="icon" onClick={() => setLang(lang === 'ja' ? 'en' : 'ja')} aria-label={t.nav.lang} title={t.nav.lang}>
          <Globe className="size-5" />
        </Button>
      </div>
    </header>
  )
}
