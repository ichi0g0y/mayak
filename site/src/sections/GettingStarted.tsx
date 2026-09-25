import { Lightbulb } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { Section } from './Section'

export function GettingStarted() {
  const t = useT()
  return (
    <Section id="start" kicker={t.start.kicker} title={t.start.title} lead={t.start.lead}>
      <div className="grid gap-10 lg:grid-cols-[1fr_1fr] lg:gap-16">
        <ol className="grid gap-6">
          {t.start.steps.map((step, i) => (
            <Reveal key={step.title} as="li" delay={i * 50} className="grid grid-cols-[2.5rem_1fr] gap-3">
              <span className="text-primary pt-0.5 font-mono text-sm font-medium">{String(i + 1).padStart(2, '0')}</span>
              <div>
                <h3 className="font-bold">{step.title}</h3>
                <p className="text-muted-foreground mt-1 text-sm leading-relaxed">{step.body}</p>
              </div>
            </Reveal>
          ))}
          <Reveal as="li" className="text-muted-foreground flex items-start gap-2 text-sm">
            <Lightbulb className="text-primary mt-0.5 size-4 shrink-0" />
            {t.start.tip}
          </Reveal>
        </ol>
        <Reveal className="lg:sticky lg:top-24 lg:self-start">
          <div className="panel overflow-hidden">
            <div className="flex items-center gap-2 border-b px-4 py-2.5">
              <img src="/assets/mayak-mark.png" alt="" width={16} height={16} className="size-4" />
              <span className="font-mono text-xs">{t.start.mock.title}</span>
            </div>
            <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2.5 px-4 py-4 text-sm">
              {t.start.mock.rows.map(([label, value]) => (
                <div key={label} className="contents">
                  <dt className="text-muted-foreground">{label}</dt>
                  <dd className="font-mono text-xs leading-5">{value}</dd>
                </div>
              ))}
            </dl>
            <div className="overflow-x-auto border-t px-4 py-3 font-mono text-[11px] leading-5">
              {t.start.mock.log.map((line) => (
                <p key={line} className="text-muted-foreground whitespace-pre">
                  {line}
                </p>
              ))}
            </div>
          </div>
        </Reveal>
      </div>
    </Section>
  )
}
