import { useAtomValue } from 'jotai'
import { Download as DownloadIcon, ExternalLink, FileCheck2, Laptop, Monitor, Terminal } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useT } from '@/i18n'
import { ARCHIVES, findAsset, formatSize, platformAtom, RELEASES_URL, releaseLoadableAtom, type Asset, type Platform } from '@/state'
import { cn } from '@/lib/utils'
import { Section } from './Section'

function AssetButton({ asset, label, primary }: { asset: Asset | undefined; label: string; primary?: boolean }) {
  return (
    <Button variant={primary ? 'default' : 'secondary'} size="sm" asChild disabled={!asset}>
      <a href={asset?.url ?? `${RELEASES_URL}/latest`} download={asset?.name}>
        <DownloadIcon />
        {label}
        {asset && <span className="opacity-70">{formatSize(asset.size)}</span>}
      </a>
    </Button>
  )
}

export function Download() {
  const t = useT()
  const release = useAtomValue(releaseLoadableAtom)
  const platform = useAtomValue(platformAtom)
  const latest = release.state === 'hasData' ? release.data : undefined
  const checksums = findAsset(latest, ARCHIVES.checksums)
  const cards: { key: Platform; icon: typeof Monitor; title: string; body: string; untested: boolean; buttons: { asset: Asset | undefined; label: string }[] }[] = [
    { key: 'windows', icon: Monitor, title: t.download.windows, body: t.download.windowsBody, untested: false, buttons: [{ asset: findAsset(latest, ARCHIVES.windows), label: ARCHIVES.windows }] },
    {
      key: 'mac',
      icon: Laptop,
      title: t.download.mac,
      body: t.download.macBody,
      untested: true,
      buttons: [
        { asset: findAsset(latest, ARCHIVES.macArm), label: t.download.appleSilicon },
        { asset: findAsset(latest, ARCHIVES.macIntel), label: t.download.intel },
      ],
    },
    { key: 'linux', icon: Terminal, title: t.download.linux, body: t.download.linuxBody, untested: true, buttons: [{ asset: findAsset(latest, ARCHIVES.linux), label: ARCHIVES.linux }] },
  ]
  const publishedAt = latest ? new Date(latest.publishedAt).toLocaleDateString(document.documentElement.lang === 'ja' ? 'ja-JP' : 'en-US') : ''
  return (
    <Section id="download" kicker={t.download.kicker} title={t.download.title} lead={t.download.lead}>
      <Reveal className="mb-4 flex flex-wrap items-center gap-3 text-sm">
        {release.state === 'loading' && <span className="text-muted-foreground">{t.download.loading}</span>}
        {release.state === 'hasError' && <span className="text-rust">{t.download.failed}</span>}
        {latest && (
          <>
            <Badge className="font-display text-sm tracking-wide">v{latest.version}</Badge>
            <span className="text-muted-foreground">{t.download.published(publishedAt)}</span>
          </>
        )}
      </Reveal>
      <div className="grid gap-3 md:grid-cols-3">
        {cards.map((card, i) => {
          const Icon = card.icon
          const primary = card.key === platform
          return (
            <Reveal key={card.key} delay={i * 70} className={cn('panel flex h-full flex-col p-5', primary && 'border-primary/50')}>
              <div className="flex items-center justify-between">
                <Icon className="text-primary/80 size-5" />
                {primary && <Badge className="font-label text-[11px]">{t.download.recommended}</Badge>}
                {card.untested && (
                  <Badge variant="outline" className="font-label text-[11px]">
                    {t.download.untested}
                  </Badge>
                )}
              </div>
              <h3 className="font-display mt-3 text-xl font-semibold tracking-wide">{card.title}</h3>
              <p className="text-muted-foreground mt-1 text-sm">{card.body}</p>
              <div className="mt-4 flex flex-wrap gap-2">
                {card.buttons.map((button) => (
                  <AssetButton key={button.label} asset={button.asset} label={button.label} primary={primary} />
                ))}
              </div>
            </Reveal>
          )
        })}
      </div>
      <Reveal className="mt-4 flex flex-wrap gap-4 text-sm">
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
