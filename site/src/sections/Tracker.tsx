import { useState } from 'react'
import { useAtomValue } from 'jotai'
import { ExternalLink, KeyRound } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { localized } from '@/lib/screenshots'
import { langAtom } from '@/state'
import { Section } from './Section'

/** Linking TarkovTracker: what it is, the three steps, and the caveats. */
export function Tracker() {
  const t = useT()
  const lang = useAtomValue(langAtom)
  const [missing, setMissing] = useState(false)
  const screenshot = localized(lang, 'tracker.png')
  return (
    <Section id="tracker" kicker={t.tracker.kicker} title={t.tracker.title} lead={t.tracker.lead}>
      <div className="grid gap-8 lg:grid-cols-[1fr_1.1fr] lg:gap-14">
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
          <div className="panel mt-8 p-5">
            <h3 className="font-bold">{t.tracker.noteTitle}</h3>
            <ul className="text-muted-foreground mt-3 grid gap-2.5 text-sm leading-relaxed">
              {t.tracker.notes.map((note) => (
                <li key={note}>{note}</li>
              ))}
            </ul>
          </div>
        </Reveal>
        {!missing && (
          <Reveal delay={80}>
            <figure className="panel overflow-hidden">
              <img key={screenshot} src={screenshot} alt="" loading="lazy" className="block w-full" onError={() => setMissing(true)} />
            </figure>
          </Reveal>
        )}
      </div>
    </Section>
  )
}
