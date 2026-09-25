import { ExternalLink, KeyRound } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { Section } from './Section'

/** Linking TarkovTracker: what it is, the three steps, and the caveats. */
export function Tracker() {
  const t = useT()
  return (
    <Section id="tracker" kicker={t.tracker.kicker} title={t.tracker.title} lead={t.tracker.lead}>
      <div className="grid gap-4 lg:grid-cols-[1.3fr_1fr]">
        <Reveal>
          <ol className="grid gap-5">
            {t.tracker.steps.map((step, i) => (
              <li key={step} className="grid grid-cols-[2.5rem_1fr] gap-3">
                <span className="text-primary pt-0.5 font-mono text-sm font-medium">{String(i + 1).padStart(2, '0')}</span>
                <p className="text-sm leading-relaxed">{step}</p>
              </li>
            ))}
          </ol>
          <a href="https://tarkovtracker.org/settings#api" rel="noopener" className="mt-6 inline-flex items-center gap-2 text-sm font-semibold underline underline-offset-4">
            <KeyRound className="size-4" />
            {t.tracker.openSettings}
            <ExternalLink className="size-3.5" />
          </a>
        </Reveal>
        <Reveal delay={80}>
          <div className="panel p-6">
            <h3 className="font-bold">{t.tracker.noteTitle}</h3>
            <ul className="text-muted-foreground mt-3 grid gap-2.5 text-sm leading-relaxed">
              {t.tracker.notes.map((note) => (
                <li key={note}>{note}</li>
              ))}
            </ul>
          </div>
        </Reveal>
      </div>
    </Section>
  )
}
