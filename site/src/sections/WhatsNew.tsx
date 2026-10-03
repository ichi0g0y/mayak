import { ArrowRight, Check, Globe, KeyRound, MapIcon, MonitorSmartphone, Target, Users } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { Section } from './Section'

const icons = [KeyRound, MapIcon, Users, Target, Globe, MonitorSmartphone]

/** What this version brought: the larger changes for someone who used MAYAK before, and the way to the changelog. */
export function WhatsNew() {
  const t = useT()
  return (
    <Section id="new" kicker={t.whatsNew.kicker} title={t.whatsNew.title} lead={t.whatsNew.lead}>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {t.whatsNew.items.map((item, i) => {
          const Icon = icons[i] ?? Check
          return (
            <Reveal key={item.title} delay={i * 60} className="panel p-5 sm:p-6">
              <div className="flex items-center gap-3">
                <Icon className="text-primary size-5 shrink-0" />
                <h3 className="font-bold">{item.title}</h3>
              </div>
              <p className="text-muted-foreground mt-3 text-sm leading-relaxed">{item.body}</p>
            </Reveal>
          )
        })}
      </div>
      <Reveal>
        <a href="/changelog" className="mt-6 inline-flex items-center gap-2 text-sm font-semibold underline underline-offset-4">
          {t.whatsNew.changelog}
          <ArrowRight className="size-4" />
        </a>
      </Reveal>
    </Section>
  )
}
