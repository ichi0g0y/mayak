import { useAtomValue } from 'jotai'
import { ArrowRight, Download } from 'lucide-react'
import { useT } from '@/i18n'
import { ARCHIVES, findAsset, formatSize, RELEASES_URL, releaseLoadableAtom } from '@/state'

/** The first screen: the logo, one line of what MAYAK is, and the installer. */
export function Hero() {
  const t = useT()
  const release = useAtomValue(releaseLoadableAtom)
  const latest = release.state === 'hasData' ? release.data : undefined
  const installer = findAsset(latest, ARCHIVES.windowsInstaller)
  const version = latest ? `v${latest.version}` : ''
  return (
    <section className="relative flex min-h-svh flex-col items-center justify-center px-4 pt-20 pb-16 text-center">
      <div className="hero-fade flex w-full flex-col items-center">
        <img src="/assets/mayak-logo-white.png" alt="MAYAK" width={512} height={512} className="w-[min(72vw,520px,46svh)] opacity-90" fetchPriority="high" />
        <a href={latest?.url ?? RELEASES_URL} rel="noopener" className="mt-2 inline-flex items-center gap-3 font-mono text-xs tracking-[0.14em] uppercase">
          <span className="bg-primary text-primary-foreground px-1.5 py-0.5 font-bold">NEW</span>
          <span className="text-muted-foreground">{latest ? t.hero.released(version) : 'Escape from Tarkov companion'}</span>
          <ArrowRight className="text-muted-foreground size-3.5" />
        </a>
        <p className="text-muted-foreground mt-6 max-w-2xl text-base leading-relaxed sm:text-lg">{t.hero.lead}</p>

        <div className="mt-8 w-full max-w-xl text-left">
          <p className="text-sm font-semibold">{latest ? t.hero.installLabel(version) : t.hero.fetching}</p>
          <a
            href={installer?.url ?? `${RELEASES_URL}/latest`}
            download={installer?.name}
            className="panel hover:border-primary/60 mt-2 flex items-center gap-4 px-4 py-3.5 transition-colors"
          >
            <span className="text-primary font-mono text-lg">&gt;</span>
            <span className="min-w-0 flex-1 truncate font-mono text-sm sm:text-base">{installer?.name ?? (latest ? t.hero.releasesFallback : ARCHIVES.windowsInstaller)}</span>
            {installer && <span className="text-muted-foreground hidden font-mono text-xs sm:inline">{formatSize(installer.size)}</span>}
            <span className="bg-primary text-primary-foreground inline-flex items-center gap-1.5 rounded-sm px-3 py-1.5 text-sm font-semibold">
              <Download className="size-4" />
              {t.hero.download}
            </span>
          </a>
          <p className="text-muted-foreground mt-2 text-xs">{t.hero.installerHint}</p>
          <a href="#start" className="mt-5 inline-flex items-center gap-2 text-sm font-semibold underline underline-offset-4">
            {t.hero.quickstart}
            <ArrowRight className="size-4" />
          </a>
        </div>
      </div>
    </section>
  )
}
