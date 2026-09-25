import { useAtomValue } from 'jotai'
import { Download, ExternalLink } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useT } from '@/i18n'
import { ARCHIVES, findAsset, formatSize, RELEASES_URL, releaseLoadableAtom, REPOSITORY } from '@/state'

export function Hero() {
  const t = useT()
  const release = useAtomValue(releaseLoadableAtom)
  const latest = release.state === 'hasData' ? release.data : undefined
  const windows = findAsset(latest, ARCHIVES.windows)
  const [titleFirst, titleSecond] = t.hero.title.split('\n')
  return (
    <section className="scanlines relative grid items-center gap-10 py-16 sm:py-24 lg:grid-cols-[1.1fr_0.9fr]">
      <div>
        <p className="font-label text-primary/80 text-sm font-semibold">{t.hero.kicker}</p>
        <h1 className="mt-4 text-4xl leading-tight font-bold tracking-tight sm:text-5xl">
          {titleFirst}
          <br />
          {titleSecond}
        </h1>
        <p className="text-muted-foreground mt-6 max-w-xl text-lg">{t.hero.lead}</p>
        <div className="mt-8 flex flex-wrap items-center gap-3">
          <Button size="lg" asChild>
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
          <Button size="lg" variant="outline" asChild>
            <a href={`https://github.com/${REPOSITORY}`} rel="noopener">
              {t.hero.github}
              <ExternalLink />
            </a>
          </Button>
        </div>
        <p className="text-muted-foreground mt-4 text-sm">{t.hero.note}</p>
        <div className="mt-6 flex flex-wrap gap-2">
          {t.hero.badges.map((badge) => (
            <Badge key={badge} variant="outline" className="border-primary/40 text-primary/90">
              {badge}
            </Badge>
          ))}
        </div>
      </div>
      <div className="relative mx-auto w-full max-w-md">
        <div className="bracket bg-card/60 rounded-xl border p-8 sm:p-10">
          <img src="/assets/mayak-logo-white.png" alt="MAYAK" className="mx-auto w-full max-w-xs drop-shadow-[0_0_40px_rgba(205,197,173,0.25)]" />
          <div className="font-label text-muted-foreground mt-4 flex items-center justify-between text-xs">
            <span>MAYAK · МАЯК</span>
            <span>{latest ? `${t.hero.latest} v${latest.version}` : '—'}</span>
          </div>
        </div>
      </div>
    </section>
  )
}
