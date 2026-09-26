import { useAtom, useAtomValue } from 'jotai'
import { ArrowRight, Download } from 'lucide-react'
import { useT } from '@/i18n'
import { macArchAtom, useDirectDownload } from '@/hooks/useDirectDownload'
import { formatSize, releaseLoadableAtom } from '@/state'
import { cn } from '@/lib/utils'

/** The first screen: the logo, one line of what MAYAK is, and the download for the visitor's OS. */
export function Hero() {
  const t = useT()
  const release = useAtomValue(releaseLoadableAtom)
  const latest = release.state === 'hasData' ? release.data : undefined
  const direct = useDirectDownload()
  const [macArch, setMacArch] = useAtom(macArchAtom)
  const version = latest ? `v${latest.version}` : ''
  return (
    <section className="relative flex min-h-svh flex-col items-center justify-center px-4 pt-20 pb-16 text-center">
      <div className="hero-fade flex w-full flex-col items-center">
        <img src="/assets/mayak-logo-white.png" alt="MAYAK" width={512} height={512} className="w-[min(72vw,520px,46svh)] opacity-90" fetchPriority="high" />
        <p className="text-muted-foreground -mt-6 font-mono text-xs tracking-[0.18em] uppercase">{t.hero.tagline}</p>
        <p className="text-muted-foreground mt-9 max-w-2xl text-base leading-relaxed sm:text-lg">{t.hero.lead}</p>

        <div className="mt-8 w-full max-w-xl text-left">
          <p className="text-sm font-semibold">{latest ? t.hero.installLabel(version, t.download[direct.platform === 'mac' ? 'mac' : direct.platform]) : t.hero.fetching}</p>
          <a href={direct.href} download={direct.download} className="panel hover:border-primary/60 mt-2 flex items-center gap-4 px-4 py-3.5 transition-colors">
            <span className="text-primary font-mono text-lg">&gt;</span>
            <span className="min-w-0 flex-1 truncate font-mono text-sm sm:text-base">{direct.asset?.name ?? t.hero.releasesFallback}</span>
            {direct.asset && <span className="text-muted-foreground hidden font-mono text-xs sm:inline">{formatSize(direct.asset.size)}</span>}
            <span className="bg-primary text-primary-foreground inline-flex items-center gap-1.5 rounded-sm px-3 py-1.5 text-sm font-semibold">
              <Download className="size-4" />
              {t.hero.download}
            </span>
          </a>
          <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
            {direct.platform === 'mac' ? (
              <div className="inline-flex overflow-hidden rounded-sm border text-xs">
                {(['arm64', 'amd64'] as const).map((arch) => (
                  <button
                    key={arch}
                    type="button"
                    onClick={() => setMacArch(arch)}
                    className={cn('px-3 py-1.5 transition-colors', macArch === arch ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:text-foreground')}
                  >
                    {arch === 'arm64' ? t.download.appleSilicon : t.download.intel}
                  </button>
                ))}
              </div>
            ) : (
              <span className="text-muted-foreground text-xs">{t.hero.hints[direct.platform]}</span>
            )}
            <a href="#download" className="text-muted-foreground hover:text-foreground text-xs underline underline-offset-4">
              {t.hero.otherPlatforms}
            </a>
          </div>
          {direct.platform === 'mac' && <p className="text-muted-foreground mt-2 text-xs">{t.hero.hints.mac}</p>}
          <a href="#start" className="mt-5 inline-flex items-center gap-2 text-sm font-semibold underline underline-offset-4">
            {t.hero.quickstart}
            <ArrowRight className="size-4" />
          </a>
        </div>
      </div>
    </section>
  )
}
