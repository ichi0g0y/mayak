import { Bell, Check, Compass, MapPinned, Package, ScanLine } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { Section } from './Section'

const icons = [MapPinned, ScanLine, Package, Compass, Bell]

export function Features() {
  const t = useT()
  return (
    <Section id="features" kicker={t.features.kicker} title={t.features.title} lead={t.features.lead}>
      <div className="grid gap-4 sm:grid-cols-2">
        {t.features.items.map((item, i) => {
          const Icon = icons[i] ?? Check
          return (
            <Reveal key={item.title} delay={i * 60} className="panel hover:border-primary/40 p-6 transition-colors sm:p-7">
              <div className="flex items-center gap-3">
                <Icon className="text-primary size-5 shrink-0" />
                <h3 className="text-xl font-bold">{item.title}</h3>
              </div>
              <p className="text-muted-foreground mt-1.5 font-mono text-xs">
                <span className="text-primary">reads</span> {item.reads}
              </p>
              <p className="text-muted-foreground mt-4 text-sm leading-relaxed">{item.body}</p>
            </Reveal>
          )
        })}
      </div>
      <Reveal className="mt-6 grid gap-2 sm:grid-cols-2">
        {t.features.more.map((line) => (
          <p key={line} className="text-muted-foreground flex items-start gap-2 text-sm">
            <Check className="text-primary mt-0.5 size-4 shrink-0" />
            {line}
          </p>
        ))}
      </Reveal>
    </Section>
  )
}
