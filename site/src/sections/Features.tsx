import { Check } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { useT } from '@/i18n'
import { Section } from './Section'

export function Features() {
  const t = useT()
  return (
    <Section id="features" kicker={t.features.kicker} title={t.features.title} lead={t.features.lead}>
      <div className="grid border-t border-l sm:grid-cols-2">
        {t.features.items.map((item, i) => (
          <Reveal key={item.title} delay={i * 60} className="flex flex-col border-r border-b p-6 sm:p-8">
            <h3 className="text-xl font-bold">{item.title}</h3>
            <p className="text-muted-foreground mt-1 font-mono text-xs">
              <span className="text-primary">reads</span> {item.reads}
            </p>
            <p className="text-muted-foreground mt-4 flex-1 text-sm leading-relaxed">{item.body}</p>
            <p className="bg-card mt-5 overflow-x-auto border px-3 py-2 font-mono text-xs whitespace-nowrap">{item.example}</p>
          </Reveal>
        ))}
      </div>
      <Reveal className="mt-8 grid gap-2 sm:grid-cols-2">
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
