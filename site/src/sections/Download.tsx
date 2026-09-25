import { useAtomValue } from 'jotai'
import { Download as DownloadIcon, ExternalLink, FileCheck2, Laptop, Monitor, Terminal } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useT } from '@/i18n'
import { ARCHIVES, findAsset, formatSize, platformAtom, RELEASES_URL, releaseLoadableAtom, type Asset, type Platform } from '@/state'
import { cn } from '@/lib/utils'
import { Reveal } from '@/components/Reveal'
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
      <div className="mb-4 flex flex-wrap items-center gap-3 text-sm">
        {release.state === 'loading' && <span className="text-muted-foreground">{t.download.loading}</span>}
        {release.state === 'hasError' && <span className="text-rust">{t.download.failed}</span>}
        {latest && (
          <>
            <Badge>v{latest.version}</Badge>
            <span className="text-muted-foreground">{t.download.published(publishedAt)}</span>
          </>
        )}
      </div>
      <div className="grid gap-4 md:grid-cols-3">
        {cards.map((card) => {
          const Icon = card.icon
          const primary = card.key === platform
          return (
            <Reveal key={card.key} delay={cards.indexOf(card) * 80}>
            <Card className={cn('bg-card/80 h-full', primary && 'bracket border-primary/50')}>
              <CardHeader>
                <div className="flex items-center justify-between">
                  <Icon className="text-primary size-6" />
                  {primary && <Badge>{t.download.recommended}</Badge>}
                  {card.untested && <Badge variant="outline">{t.download.untested}</Badge>}
                </div>
                <CardTitle className="mt-2 text-lg">{card.title}</CardTitle>
                <CardDescription>{card.body}</CardDescription>
              </CardHeader>
              <CardContent className="flex flex-wrap gap-2">
                {card.buttons.map((button) => (
                  <AssetButton key={button.label} asset={button.asset} label={button.label} primary={primary} />
                ))}
              </CardContent>
            </Card>
            </Reveal>
          )
        })}
      </div>
      <div className="mt-4 flex flex-wrap gap-4 text-sm">
        <a href={checksums?.url ?? RELEASES_URL} className="text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5">
          <FileCheck2 className="size-4" />
          {t.download.checksums}
        </a>
        <a href={RELEASES_URL} rel="noopener" className="text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5">
          <ExternalLink className="size-4" />
          {t.download.releases}
        </a>
      </div>
    </Section>
  )
}
