import { useState } from 'react'
import { useAtomValue } from 'jotai'
import { ArrowRight, Download } from 'lucide-react'
import { useT } from '@/i18n'
import { ARCHIVES, findAsset, formatSize, RELEASES_URL, releaseLoadableAtom } from '@/state'
import { cn } from '@/lib/utils'

/** The headline, the install box and the logo. */
export function Hero() {
  const t = useT()
  const release = useAtomValue(releaseLoadableAtom)
  const latest = release.state === 'hasData' ? release.data : undefined
  const [choice, setChoice] = useState<'installer' | 'portable'>('installer')
  const installer = findAsset(latest, ARCHIVES.windowsInstaller)
  const zip = findAsset(latest, ARCHIVES.windows)
  // A release without an installer (0.1.0) offers the zip only.
  const installerAvailable = !latest || !!installer
  const kind = installerAvailable ? choice : 'portable'
  const asset = kind === 'installer' ? installer : zip
  const version = latest ? `v${latest.version}` : ''
  const afterLines = t.hero.title.after.split('\n')
  return (
    <section className="grid gap-12 border-b px-6 py-16 sm:px-10 sm:py-24 lg:grid-cols-[1.15fr_0.85fr] lg:items-center">
      <div>
        <a href={latest?.url ?? RELEASES_URL} rel="noopener" className="inline-flex items-center gap-3 font-mono text-xs tracking-[0.14em] uppercase">
          <span className="bg-primary text-primary-foreground px-1.5 py-0.5 font-bold">NEW</span>
          <span className="text-muted-foreground">{latest ? t.hero.released(version) : 'MAYAK'}</span>
          <ArrowRight className="text-muted-foreground size-3.5" />
        </a>
        <h1 className="headline mt-6 text-[2.6rem] sm:text-6xl lg:text-[4.25rem]">
          {t.hero.title.before}
          <span className="text-primary whitespace-nowrap">{t.hero.title.accent}</span>
          {afterLines.map((line, i) => (
            <span key={i}>
              {i > 0 && <br />}
              {line}
            </span>
          ))}
        </h1>
        <p className="text-muted-foreground mt-6 max-w-xl text-lg leading-relaxed">{t.hero.lead}</p>

        <div className="mt-10 max-w-xl">
          <p className="text-sm font-semibold">{latest ? t.hero.installLabel(version) : t.hero.fetching}</p>
          <a
            href={asset?.url ?? `${RELEASES_URL}/latest`}
            download={asset?.name}
            className="bg-card hover:border-primary/60 group mt-2 flex items-center gap-4 border px-4 py-3.5 transition-colors"
          >
            <span className="text-primary font-mono text-lg">&gt;</span>
            <span className="min-w-0 flex-1 truncate font-mono text-sm sm:text-base">{asset?.name ?? (latest ? ARCHIVES.windowsInstaller : t.hero.releasesFallback)}</span>
            {asset && <span className="text-muted-foreground hidden font-mono text-xs sm:inline">{formatSize(asset.size)}</span>}
            <span className="bg-primary text-primary-foreground inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-semibold">
              <Download className="size-4" />
              {t.hero.download}
            </span>
          </a>
          <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
            <div className="inline-flex border text-xs">
              {(['installer', 'portable'] as const).map((option) => (
                <button
                  key={option}
                  type="button"
                  disabled={option === 'installer' && !installerAvailable}
                  onClick={() => setChoice(option)}
                  className={cn(
                    'px-3 py-1.5 transition-colors disabled:cursor-not-allowed disabled:opacity-40',
                    kind === option ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:text-foreground',
                  )}
                >
                  {option === 'installer' ? t.hero.installer : t.hero.portable}
                </button>
              ))}
            </div>
            <span className="text-muted-foreground text-xs">{kind === 'installer' ? t.hero.installerHint : t.hero.portableHint}</span>
          </div>
          <a href="#start" className="mt-6 inline-flex items-center gap-2 text-sm font-semibold underline underline-offset-4">
            {t.hero.quickstart}
            <ArrowRight className="size-4" />
          </a>
        </div>
      </div>
      <div className="flex items-center justify-center">
        <img src="/assets/mayak-logo-white.png" alt="MAYAK" width={512} height={512} className="w-[min(70vw,440px)]" fetchPriority="high" />
      </div>
    </section>
  )
}
