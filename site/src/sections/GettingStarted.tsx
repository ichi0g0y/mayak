import { useState } from 'react'
import { useAtomValue } from 'jotai'
import { Lightbulb } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import type { Lang } from '@/i18n/languages'
import { localized, shared } from '@/lib/screenshots'
import { langAtom } from '@/state'
import { Section } from './Section'

/** The screenshots beside each step; a missing file is skipped. */
function stepImages(lang: Lang): Record<number, string[]> {
  return {
    1: [shared('setup-1.png')],
    2: [shared('setup-2.png')],
    3: [localized(lang, 'setup-3.png')],
    4: [shared('setup-4.png')],
    5: [localized(lang, 'setup-5.png'), localized(lang, 'setup-5-2.png')],
    6: [localized(lang, 'setup-6.png')],
  }
}

function Shot({ src, alt }: { src: string; alt: string }) {
  const [missing, setMissing] = useState(false)
  if (missing) return null
  return (
    <figure className="panel overflow-hidden">
      <img src={src} alt={alt} loading="lazy" className="block w-full" onError={() => setMissing(true)} />
    </figure>
  )
}

export function GettingStarted() {
  const t = useT()
  const lang = useAtomValue(langAtom)
  const images = stepImages(lang)
  return (
    <Section id="start" kicker={t.start.kicker} title={t.start.title} lead={t.start.lead}>
      <ol className="grid gap-12 sm:gap-14">
        {t.start.steps.map((step, i) => (
          <Reveal key={step.title} as="li" className="grid gap-5 border-t pt-8 lg:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)] lg:gap-10">
            <div className="grid grid-cols-[2.5rem_1fr] gap-3">
              <span className="text-primary pt-1 font-mono text-sm font-medium">{String(i + 1).padStart(2, '0')}</span>
              <div>
                <h3 className="text-lg font-bold">{step.title}</h3>
                <p className="text-muted-foreground mt-2 text-sm leading-relaxed">{step.body}</p>
              </div>
            </div>
            <div className="grid gap-4">
              {(images[i + 1] ?? []).map((src) => (
                <Shot key={src} src={src} alt={step.title} />
              ))}
            </div>
          </Reveal>
        ))}
      </ol>
      <Reveal>
        <p className="text-muted-foreground mt-10 flex items-start gap-2 text-sm">
          <Lightbulb className="text-primary mt-0.5 size-4 shrink-0" />
          {t.start.tip}
        </p>
      </Reveal>
    </Section>
  )
}
