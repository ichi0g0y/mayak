import { useEffect, useRef, useState } from 'react'
import { Lightbulb } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { cn } from '@/lib/utils'
import { Section } from './Section'

/** The screenshot for a step, when one has been added under public/screenshots. */
const screenshot = (step: number) => `/screenshots/setup-${step}.png`

export function GettingStarted() {
  const t = useT()
  const steps = t.start.steps
  const [active, setActive] = useState(0)
  // Which step screenshots exist; a missing file hides the panel for that step.
  const [available, setAvailable] = useState<Record<number, boolean>>({})
  const items = useRef<(HTMLLIElement | null)[]>([])

  useEffect(() => {
    steps.forEach((_, i) => {
      const image = new Image()
      image.onload = () => setAvailable((current) => ({ ...current, [i]: true }))
      image.onerror = () => setAvailable((current) => ({ ...current, [i]: false }))
      image.src = screenshot(i + 1)
    })
  }, [steps.length])

  // The step nearest the middle of the viewport is the active one.
  useEffect(() => {
    if (!('IntersectionObserver' in window)) return
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) setActive(Number((entry.target as HTMLElement).dataset.step))
        }
      },
      { rootMargin: '-35% 0px -55% 0px', threshold: 0 },
    )
    items.current.forEach((item) => item && observer.observe(item))
    return () => observer.disconnect()
  }, [steps.length])

  const anyImage = Object.values(available).some(Boolean)
  const shown = available[active] ? active : Object.keys(available).map(Number).find((i) => available[i] && i <= active)
  return (
    <Section id="start" kicker={t.start.kicker} title={t.start.title} lead={t.start.lead}>
      <div className={cn('grid gap-10', anyImage && 'lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)] lg:gap-14')}>
        <ol className="grid gap-7">
          {steps.map((step, i) => (
            <Reveal key={step.title} as="li" delay={i * 40}>
              <div
                ref={(node) => {
                  items.current[i] = node as HTMLLIElement | null
                }}
                data-step={i}
                onMouseEnter={() => setActive(i)}
                className={cn('grid grid-cols-[2.5rem_1fr] gap-3 transition-opacity', anyImage && active !== i && 'opacity-70')}
              >
                <span className="text-primary pt-0.5 font-mono text-sm font-medium">{String(i + 1).padStart(2, '0')}</span>
                <div>
                  <h3 className="font-bold">{step.title}</h3>
                  <p className="text-muted-foreground mt-1 text-sm leading-relaxed">{step.body}</p>
                </div>
              </div>
            </Reveal>
          ))}
          <Reveal as="li" className="text-muted-foreground flex items-start gap-2 text-sm">
            <Lightbulb className="text-primary mt-0.5 size-4 shrink-0" />
            {t.start.tip}
          </Reveal>
        </ol>
        {anyImage && (
          <Reveal className="hidden lg:block">
            <figure className="panel sticky top-24 overflow-hidden">
              {shown !== undefined && <img src={screenshot(shown + 1)} alt={steps[shown].title} className="block w-full" />}
              {shown !== undefined && <figcaption className="text-muted-foreground border-t px-4 py-2.5 text-xs">{`${String(shown + 1).padStart(2, '0')}  ${steps[shown].title}`}</figcaption>}
            </figure>
          </Reveal>
        )}
      </div>
    </Section>
  )
}
