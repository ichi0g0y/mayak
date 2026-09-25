import { useAtomValue } from 'jotai'
import { Download as DownloadIcon, ExternalLink, FileCheck2 } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { Badge } from '@/components/ui/badge'
import { useT } from '@/i18n'
import { ARCHIVES, findAsset, formatSize, platformAtom, RELEASES_URL, releaseLoadableAtom, type Asset, type Platform } from '@/state'
import { cn } from '@/lib/utils'
import { Section } from './Section'

function AssetLink({ asset, label, primary }: { asset: Asset | undefined; label: string; primary?: boolean }) {
  return (
    <a
      href={asset?.url ?? `${RELEASES_URL}/latest`}
      download={asset?.name}
      className={cn(
        'flex items-center gap-3 rounded-sm border px-3 py-2 text-sm transition-colors',
        primary ? 'bg-primary text-primary-foreground border-primary hover:bg-primary/90' : 'bg-secondary/60 hover:border-primary/60',
        !asset && 'opacity-60',
      )}
    >
      <DownloadIcon className="size-4 shrink-0" />
      <span className="min-w-0 flex-1 truncate font-medium">{label}</span>
      {asset && <span className={cn('font-mono text-xs', primary ? 'text-primary-foreground/70' : 'text-muted-foreground')}>{formatSize(asset.size)}</span>}
    </a>
  )
}

export function Download() {
  const t = useT()
  const release = useAtomValue(releaseLoadableAtom)
  const platform = useAtomValue(platformAtom)
  const latest = release.state === 'hasData' ? release.data : undefined
  const checksums = findAsset(latest, ARCHIVES.checksums)
  const cards: { key: Platform; title: string; body: string; steps: string[]; untested: boolean; links: { asset: Asset | undefined; label: string; primary?: boolean }[] }[] = [
    {
      key: 'windows',
      title: t.download.windows,
      body: t.download.windowsBody,
      steps: t.download.windowsSteps,
      untested: false,
      links: [
        { asset: findAsset(latest, ARCHIVES.windowsInstaller), label: t.download.installer, primary: true },
        { asset: findAsset(latest, ARCHIVES.windows), label: t.download.portable },
      ],
    },
    {
      key: 'mac',
      title: t.download.mac,
      body: t.download.macBody,
      steps: t.download.macSteps,
      untested: true,
      links: [
        { asset: findAsset(latest, ARCHIVES.macArm), label: t.download.appleSilicon, primary: true },
        { asset: findAsset(latest, ARCHIVES.macIntel), label: t.download.intel, primary: true },
      ],
    },
    {
      key: 'linux',
      title: t.download.linux,
      body: t.download.linuxBody,
      steps: t.download.linuxSteps,
      untested: true,
      links: [{ asset: findAsset(latest, ARCHIVES.linux), label: ARCHIVES.linux, primary: true }],
    },
  ]
  const publishedAt = latest ? new Date(latest.publishedAt).toLocaleDateString(document.documentElement.lang === 'ja' ? 'ja-JP' : 'en-US') : ''
  const aside = (
    <div className="flex flex-wrap items-center gap-3 text-sm lg:justify-end">
      {release.state === 'loading' && <span className="text-muted-foreground">{t.download.loading}</span>}
      {release.state === 'hasError' && <span className="text-rust">{t.download.failed}</span>}
      {latest && (
        <>
          <Badge className="font-mono">v{latest.version}</Badge>
          <span className="text-muted-foreground">{t.download.published(publishedAt)}</span>
        </>
      )}
    </div>
  )
  return (
    <Section id="download" kicker={t.download.kicker} title={t.download.title} lead={t.download.lead} aside={aside}>
      <div className="grid gap-4 md:grid-cols-3">
        {cards.map((card, i) => {
          const mine = card.key === platform
          return (
            <Reveal key={card.key} delay={i * 60} className={cn('panel flex flex-col p-6', mine && 'border-primary/50')}>
              <div className="flex items-center justify-between gap-2">
                <h3 className="text-xl font-bold">{card.title}</h3>
                {mine && <Badge>{t.download.recommended}</Badge>}
                {card.untested && <Badge variant="outline">{t.download.untested}</Badge>}
              </div>
              <p className="text-muted-foreground mt-2 text-sm">{card.body}</p>
              <div className="mt-4 grid gap-2">
                {card.links.map((link) => (
                  <AssetLink key={link.label} asset={link.asset} label={link.label} primary={link.primary && !!link.asset} />
                ))}
              </div>
              <ol className="text-muted-foreground mt-4 grid list-decimal gap-1.5 pl-5 text-xs leading-relaxed">
                {card.steps.map((step) => (
                  <li key={step}>{step}</li>
                ))}
              </ol>
            </Reveal>
          )
        })}
      </div>
      <Reveal className="mt-5 flex flex-wrap gap-5 text-sm">
        <a href={checksums?.url ?? RELEASES_URL} className="text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5">
          <FileCheck2 className="size-4" />
          {t.download.checksums}
        </a>
        <a href={RELEASES_URL} rel="noopener" className="text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5">
          <ExternalLink className="size-4" />
          {t.download.releases}
        </a>
      </Reveal>
    </Section>
  )
}
