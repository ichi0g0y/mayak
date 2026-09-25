import { Lightbulb } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { Section } from './Section'

export function GettingStarted() {
  const t = useT()
  return (
    <Section id="start" kicker={t.start.kicker} title={t.start.title} lead={t.start.lead}>
      <ol className="grid gap-3 md:grid-cols-2">
        {t.start.steps.map((step, i) => (
          <Reveal key={step.title} as="li" delay={i * 70} className="panel hover:border-primary/40 p-5 transition-colors">
            <div className="flex items-baseline gap-3">
              <span className="font-display text-primary/70 text-lg font-semibold tabular-nums">{String(i + 1).padStart(2, '0')}</span>
              <h3 className="font-semibold">{step.title}</h3>
            </div>
            <p className="text-muted-foreground mt-2 text-sm leading-relaxed">{step.body}</p>
          </Reveal>
        ))}
      </ol>
      <Reveal>
        <p className="text-muted-foreground mt-6 flex items-start gap-2 text-sm">
          <Lightbulb className="text-primary/80 mt-0.5 size-4 shrink-0" />
          {t.start.tip}
        </p>
      </Reveal>
    </Section>
  )
}
