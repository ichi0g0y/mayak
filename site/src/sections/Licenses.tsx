import { ExternalLink } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { REPOSITORY } from '@/state'
import { Section } from './Section'

export function Licenses() {
  const t = useT()
  return (
    <Section id="license" kicker={t.license.kicker} title={t.license.title} lead={t.license.lead}>
      <Reveal>
        <div className="grid border-t border-l lg:grid-cols-[1fr_1.4fr]">
          <div className="border-r border-b p-6">
            <h3 className="font-bold">{t.license.appTitle}</h3>
            <p className="text-muted-foreground mt-3 text-sm leading-relaxed">{t.license.appBody}</p>
            <div className="mt-4 flex flex-wrap gap-4 text-sm">
              <a href={`https://github.com/${REPOSITORY}/blob/main/LICENSE`} rel="noopener" className="inline-flex items-center gap-1 underline underline-offset-4">
                LICENSE <ExternalLink className="size-3.5" />
              </a>
              <a href={`https://github.com/${REPOSITORY}`} rel="noopener" className="inline-flex items-center gap-1 underline underline-offset-4">
                {t.footer.source} <ExternalLink className="size-3.5" />
              </a>
            </div>
          </div>
          <div className="border-r border-b p-6">
            <h3 className="font-bold">{t.license.thirdTitle}</h3>
            <ul className="mt-3 grid gap-3 sm:grid-cols-2">
              {t.license.third.map((item) => (
                <li key={item.name} className="text-sm">
                  <a href={item.url} rel="noopener" className="inline-flex items-center gap-1 font-medium hover:underline">
                    {item.name} <ExternalLink className="text-muted-foreground size-3" />
                  </a>
                  <span className="text-muted-foreground block text-xs leading-relaxed">{item.note}</span>
                </li>
              ))}
            </ul>
            <p className="text-muted-foreground mt-4 border-t pt-3 text-xs leading-relaxed">{t.license.noticesNote}</p>
          </div>
        </div>
      </Reveal>
    </Section>
  )
}
