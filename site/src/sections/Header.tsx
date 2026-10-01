import { useEffect, useState } from 'react'
import { useAtom } from 'jotai'
import { Globe } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger } from '@/components/ui/select'
import { isLang, languages, useT } from '@/i18n'
import { langAtom, REPOSITORY } from '@/state'
import { useDirectDownload } from '@/hooks/useDirectDownload'
import { cn } from '@/lib/utils'

/** The top bar. On the home page it is transparent over the logo and turns
 *  solid once the page scrolls; on other pages it is always solid. */
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
  const direct = useDirectDownload()
  const links: [string, string][] = [
    [`${prefix}#features`, t.nav.features],
    [`${prefix}#start`, t.nav.start],
    [`${prefix}#squad`, t.nav.squad],
    [`${prefix}#safety`, t.nav.safety],
    [`${prefix}#faq`, t.nav.faq],
    ['/changelog', t.nav.changelog],
  ]
  return (
    <header className={cn('fixed inset-x-0 top-0 z-20 transition-colors duration-300', scrolled ? 'bg-background/85 border-b backdrop-blur' : 'border-b border-transparent')}>
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between gap-6 px-4 sm:px-6">
        <a href="/" className={cn('flex items-center gap-2.5 transition-opacity duration-300', scrolled ? 'opacity-100' : 'opacity-0')} aria-hidden={!scrolled}>
          <img src="/assets/mayak-mark-white.png" alt="" width={28} height={28} className="size-7" />
          <span className="text-lg font-extrabold tracking-tight">MAYAK</span>
        </a>
        <nav className="hidden items-center gap-5 text-sm whitespace-nowrap lg:flex">
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
          <Button asChild size="sm" className="font-semibold">
            <a href={direct.asset ? direct.href : `${prefix}#download`} download={direct.download}>{t.nav.downloadButton}</a>
          </Button>
          <Select value={lang} onValueChange={(value) => isLang(value) && setLang(value)}>
            <SelectTrigger size="sm" chevron={false} className="relative size-8 justify-center bg-transparent p-0" aria-label={t.nav.language}>
              <Globe className="size-5" />
              <span className="bg-background absolute right-0 bottom-0 rounded-sm px-0.5 font-mono text-[9px] leading-tight font-bold">{lang.charAt(0).toUpperCase() + lang.slice(1)}</span>
            </SelectTrigger>
            <SelectContent align="end">
              {languages.map((language) => (
                <SelectItem key={language.code} value={language.code} lang={language.html}>
                  {language.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    </header>
  )
}
