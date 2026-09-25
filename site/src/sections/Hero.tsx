import { useAtomValue } from 'jotai'
import { ChevronDown, Download, ExternalLink } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useT } from '@/i18n'
import { ARCHIVES, findAsset, formatSize, RELEASES_URL, releaseLoadableAtom, REPOSITORY } from '@/state'

export function Hero() {
  const t = useT()
  const release = useAtomValue(releaseLoadableAtom)
  const latest = release.state === 'hasData' ? release.data : undefined
  const windows = findAsset(latest, ARCHIVES.windows)
  return (
    <section className="relative flex min-h-svh flex-col items-center justify-center overflow-hidden px-4 pt-20 pb-24 text-center">
      <div className="beam" aria-hidden="true" />
      <div className="hero-fade relative flex flex-col items-center">
        <img
          src="/assets/mayak-logo-white.png"
          alt="MAYAK"
          width={512}
          height={512}
          className="w-[min(72vw,520px,46svh)] drop-shadow-[0_0_60px_rgba(212,204,178,0.18)]"
          fetchPriority="high"
        />
        <p className="font-label text-primary/80 mt-2 text-sm sm:text-base">{t.hero.kicker}</p>
        <p className="text-muted-foreground mt-6 max-w-xl text-base leading-relaxed sm:text-lg">{t.hero.lead}</p>
        <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
          <Button size="lg" asChild className="h-12 px-7 text-base">
            <a href={windows?.url ?? `${RELEASES_URL}/latest`} download={windows ? ARCHIVES.windows : undefined}>
              <Download />
              {t.hero.download}
              {latest && (
                <span className="text-primary-foreground/70 text-sm font-normal">
                  v{latest.version}
                  {windows ? ` · ${formatSize(windows.size)}` : ''}
                </span>
              )}
            </a>
          </Button>
          <Button size="lg" variant="outline" asChild className="h-12 bg-transparent px-7 text-base">
            <a href={`https://github.com/${REPOSITORY}`} rel="noopener">
              {t.hero.github}
              <ExternalLink />
            </a>
          </Button>
        </div>
        <p className="text-muted-foreground/80 mt-4 text-xs sm:text-sm">{t.hero.note}</p>
      </div>
      <a href="#features" className="text-muted-foreground absolute bottom-5 flex flex-col items-center gap-1 text-xs" aria-label="Scroll">
        <span className="font-label">scroll</span>
        <ChevronDown className="nudge size-5" />
      </a>
      <div className="from-background pointer-events-none absolute inset-x-0 bottom-0 h-32 bg-gradient-to-t to-transparent" aria-hidden="true" />
    </section>
  )
}
