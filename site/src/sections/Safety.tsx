import { ShieldCheck } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { Section } from './Section'

export function Safety() {
  const t = useT()
  return (
    <Section id="safety" kicker={t.safety.kicker} title={t.safety.title} lead={t.safety.lead}>
      <Reveal>
        <ul className="grid border-t border-l sm:grid-cols-2 lg:grid-cols-3">
          {t.safety.items.map((item) => (
            <li key={item} className="flex items-start gap-3 border-r border-b p-5">
              <ShieldCheck className="text-olive mt-0.5 size-5 shrink-0" />
              <span className="text-sm leading-relaxed">{item}</span>
            </li>
          ))}
        </ul>
        <p className="text-muted-foreground mt-6 max-w-2xl text-xs leading-relaxed">{t.safety.note}</p>
      </Reveal>
    </Section>
  )
}
