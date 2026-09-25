import { useEffect, useState } from 'react'
import { useAtom } from 'jotai'
import { ExternalLink, Languages } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useT } from '@/i18n'
import { langAtom, REPOSITORY } from '@/state'
import { cn } from '@/lib/utils'

export function Header() {
  const t = useT()
  const [lang, setLang] = useAtom(langAtom)
  // Transparent over the hero, solid once the page scrolls.
  const [scrolled, setScrolled] = useState(false)
  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 40)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])
  const links: [string, string][] = [
    ['#features', t.nav.features],
    ['#start', t.nav.start],
    ['#safety', t.nav.safety],
    ['#download', t.nav.download],
    ['#faq', t.nav.faq],
    ['#license', t.nav.license],
  ]
  return (
    <header className={cn('fixed inset-x-0 top-0 z-20 transition-colors duration-300', scrolled ? 'bg-background/85 border-b backdrop-blur' : 'border-b border-transparent')}>
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between gap-4 px-4 sm:px-6">
        <a href="#" className={cn('flex items-center gap-2.5 transition-opacity duration-300', scrolled ? 'opacity-100' : 'opacity-0')} aria-hidden={!scrolled}>
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
        <Button variant="outline" size="sm" className="bg-transparent" onClick={() => setLang(lang === 'ja' ? 'en' : 'ja')} aria-label="Switch language">
          <Languages />
          {t.nav.lang}
        </Button>
      </div>
    </header>
  )
}
