import { ExternalLink, Scale } from 'lucide-react'
import { useT } from '@/i18n'
import { REPOSITORY } from '@/state'
import { Section } from './Section'

export function Licenses() {
  const t = useT()
  return (
    <Section id="license" kicker={t.license.kicker} title={t.license.title} lead={t.license.lead}>
      <div className="grid gap-4 lg:grid-cols-[1fr_1.4fr]">
        <div className="bracket bg-card/70 rounded-xl border p-6">
          <div className="flex items-center gap-2">
            <Scale className="text-primary size-5" />
            <h3 className="font-semibold">{t.license.appTitle}</h3>
          </div>
          <p className="text-muted-foreground mt-3 text-sm leading-relaxed">{t.license.appBody}</p>
          <div className="mt-4 flex flex-wrap gap-4 text-sm">
            <a href={`https://github.com/${REPOSITORY}/blob/main/LICENSE`} rel="noopener" className="text-primary inline-flex items-center gap-1 hover:underline">
              LICENSE <ExternalLink className="size-3.5" />
            </a>
            <a href={`https://github.com/${REPOSITORY}`} rel="noopener" className="text-primary inline-flex items-center gap-1 hover:underline">
              {t.footer.source} <ExternalLink className="size-3.5" />
            </a>
          </div>
        </div>
        <div className="bg-card/70 rounded-xl border p-6">
          <h3 className="font-semibold">{t.license.thirdTitle}</h3>
          <ul className="mt-3 grid gap-3 sm:grid-cols-2">
            {t.license.third.map((item) => (
              <li key={item.name} className="text-sm">
                <a href={item.url} rel="noopener" className="text-foreground inline-flex items-center gap-1 font-medium hover:underline">
                  {item.name} <ExternalLink className="text-muted-foreground size-3" />
                </a>
                <span className="text-muted-foreground block text-xs leading-relaxed">{item.note}</span>
              </li>
            ))}
          </ul>
          <p className="text-muted-foreground mt-4 border-t pt-3 text-xs leading-relaxed">{t.license.noticesNote}</p>
        </div>
      </div>
    </Section>
  )
}
