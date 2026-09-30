import { useState } from 'react'
import { useAtomValue } from 'jotai'
import { Lock, Users } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { localized } from '@/lib/screenshots'
import { langAtom } from '@/state'
import { Section } from './Section'

/** Squads: the three steps to one, what it offers, and what it sends. */
export function Squad() {
  const t = useT()
  const lang = useAtomValue(langAtom)
  const [missing, setMissing] = useState(false)
  const screenshot = localized(lang, 'squad.png')
  return (
    <Section id="squad" kicker={t.squad.kicker} title={t.squad.title} lead={t.squad.lead}>
      <Reveal>
        <ol className="grid gap-4 md:grid-cols-3">
          {t.squad.steps.map((step, i) => (
            <li key={step} className="panel grid grid-cols-[2.5rem_1fr] gap-3 p-5">
              <span className="text-primary pt-0.5 font-mono text-sm font-medium">{String(i + 1).padStart(2, '0')}</span>
              <p className="text-sm leading-relaxed">{step}</p>
            </li>
          ))}
        </ol>
      </Reveal>
      {!missing && (
        <Reveal className="mt-6">
          <figure className="panel overflow-hidden">
            <img key={screenshot} src={screenshot} alt="" loading="lazy" className="block w-full" onError={() => setMissing(true)} />
          </figure>
        </Reveal>
      )}
      <div className="mt-6 grid gap-4 lg:grid-cols-2">
        <Reveal className="panel p-5 sm:p-6">
          <h3 className="flex items-center gap-2 font-bold">
            <Users className="text-primary size-4" />
            {t.squad.noteTitle}
          </h3>
          <ul className="text-muted-foreground mt-3 grid gap-2.5 text-sm leading-relaxed">
            {t.squad.notes.map((note) => (
              <li key={note}>{note}</li>
            ))}
          </ul>
        </Reveal>
        <Reveal delay={80} className="panel p-5 sm:p-6">
          <h3 className="flex items-center gap-2 font-bold">
            <Lock className="text-olive size-4" />
            {t.squad.privacyTitle}
          </h3>
          <ul className="text-muted-foreground mt-3 grid gap-2.5 text-sm leading-relaxed">
            {t.squad.privacy.map((note) => (
              <li key={note}>{note}</li>
            ))}
          </ul>
        </Reveal>
      </div>
    </Section>
  )
}
