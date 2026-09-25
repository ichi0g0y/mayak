import { Bell, Compass, MapPinned, Package, RefreshCw, ScanLine } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { Section } from './Section'

const icons = [MapPinned, ScanLine, Package, Compass, Bell, RefreshCw]

export function Features() {
  const t = useT()
  return (
    <Section id="features" kicker={t.features.kicker} title={t.features.title} lead={t.features.lead}>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {t.features.items.map((item, i) => {
          const Icon = icons[i]
          return (
            <Reveal key={item.title} delay={i * 70} className="panel hover:border-primary/40 flex h-full flex-col p-5 transition-colors">
              <div className="flex items-center gap-3">
                <Icon className="text-primary/80 size-5 shrink-0" />
                <h3 className="font-semibold">{item.title}</h3>
              </div>
              <p className="text-muted-foreground mt-3 text-sm leading-relaxed">{item.body}</p>
            </Reveal>
          )
        })}
      </div>
    </Section>
  )
}
