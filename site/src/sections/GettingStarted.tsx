import { Lightbulb } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { Section } from './Section'

export function GettingStarted() {
  const t = useT()
  return (
    <Section id="start" kicker={t.start.kicker} title={t.start.title} lead={t.start.lead}>
      <ol className="grid gap-4 md:grid-cols-2">
        {t.start.steps.map((step, i) => (
          <Reveal key={step.title} as="li" delay={i * 80} className="bg-card/80 hover:border-primary/40 relative rounded-xl border p-5 pl-16 transition-colors">
            <span className="font-display bg-primary text-primary-foreground absolute top-5 left-5 flex size-8 items-center justify-center rounded-md text-sm">
              {String(i + 1).padStart(2, '0')}
            </span>
            <h3 className="font-semibold">{step.title}</h3>
            <p className="text-muted-foreground mt-1.5 text-sm leading-relaxed">{step.body}</p>
          </Reveal>
        ))}
      </ol>
      <Reveal>
        <p className="text-muted-foreground mt-6 flex items-start gap-2 text-sm">
          <Lightbulb className="text-primary mt-0.5 size-4 shrink-0" />
          {t.start.tip}
        </p>
      </Reveal>
    </Section>
  )
}
