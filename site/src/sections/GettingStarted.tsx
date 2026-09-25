import { Lightbulb } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { Section } from './Section'

export function GettingStarted() {
  const t = useT()
  return (
    <Section id="start" kicker={t.start.kicker} title={t.start.title} lead={t.start.lead}>
      <ol className="grid gap-x-12 gap-y-7 md:grid-cols-2">
        {t.start.steps.map((step, i) => (
          <Reveal key={step.title} as="li" delay={i * 50} className="grid grid-cols-[2.5rem_1fr] gap-3">
            <span className="text-primary pt-0.5 font-mono text-sm font-medium">{String(i + 1).padStart(2, '0')}</span>
            <div>
              <h3 className="font-bold">{step.title}</h3>
              <p className="text-muted-foreground mt-1 text-sm leading-relaxed">{step.body}</p>
            </div>
          </Reveal>
        ))}
      </ol>
      <Reveal>
        <p className="text-muted-foreground mt-8 flex items-start gap-2 text-sm">
          <Lightbulb className="text-primary mt-0.5 size-4 shrink-0" />
          {t.start.tip}
        </p>
      </Reveal>
    </Section>
  )
}
