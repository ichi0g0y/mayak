import { ShieldCheck } from 'lucide-react'
import { useT } from '@/i18n'
import { Section } from './Section'

export function Safety() {
  const t = useT()
  return (
    <Section id="safety" kicker={t.safety.kicker} title={t.safety.title} lead={t.safety.lead}>
      <div className="bracket bg-card/70 rounded-xl border p-6 sm:p-8">
        <ul className="grid gap-3 sm:grid-cols-2">
          {t.safety.items.map((item) => (
            <li key={item} className="flex items-start gap-3">
              <ShieldCheck className="text-olive mt-0.5 size-5 shrink-0" />
              <span className="text-sm leading-relaxed">{item}</span>
            </li>
          ))}
        </ul>
        <p className="text-muted-foreground mt-6 border-t pt-4 text-xs leading-relaxed">{t.safety.note}</p>
      </div>
    </Section>
  )
}
