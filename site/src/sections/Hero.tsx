import { useAtomValue } from 'jotai'
import { Download, ExternalLink } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useT } from '@/i18n'
import { ARCHIVES, findAsset, formatSize, RELEASES_URL, releaseLoadableAtom, REPOSITORY } from '@/state'

export function Hero() {
  const t = useT()
  const release = useAtomValue(releaseLoadableAtom)
  const latest = release.state === 'hasData' ? release.data : undefined
  // The installer, or the zip while a release has none.
  const windows = findAsset(latest, ARCHIVES.windowsInstaller) ?? findAsset(latest, ARCHIVES.windows)
  return (
    <section className="relative flex min-h-svh flex-col items-center justify-center overflow-hidden px-4 pt-20 pb-24 text-center">
      <div className="hero-fade relative flex flex-col items-center">
        <img
          src="/assets/mayak-logo-white.png"
          alt="MAYAK"
          width={512}
          height={512}
          className="w-[min(72vw,520px,46svh)] opacity-90"
          fetchPriority="high"
        />
        <p className="font-label text-primary/70 mt-2 text-sm sm:text-base">{t.hero.kicker}</p>
        <p className="text-muted-foreground mt-6 max-w-xl text-base leading-relaxed sm:text-lg">{t.hero.lead}</p>
        <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
          <Button size="lg" asChild className="h-12 px-7 text-base">
            <a href={windows?.url ?? `${RELEASES_URL}/latest`} download={windows?.name}>
              <Download />
              {t.hero.download}
              {latest && (
                <span className="font-display text-primary-foreground/70 text-sm font-semibold tracking-wide">
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
    </section>
  )
}
