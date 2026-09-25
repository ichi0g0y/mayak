import { Bell, Compass, MapPinned, Package, RefreshCw, ScanLine } from 'lucide-react'
import { Reveal } from '@/components/Reveal'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useT } from '@/i18n'
import { Section } from './Section'

const icons = [MapPinned, ScanLine, Package, Compass, Bell, RefreshCw]

export function Features() {
  const t = useT()
  return (
    <Section id="features" kicker={t.features.kicker} title={t.features.title} lead={t.features.lead}>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {t.features.items.map((item, i) => {
          const Icon = icons[i]
          return (
            <Reveal key={item.title} delay={i * 80}>
              <Card className="bracket bg-card/80 hover:border-primary/40 h-full transition-colors">
                <CardHeader>
                  <div className="bg-primary/10 text-primary mb-2 flex size-10 items-center justify-center rounded-md">
                    <Icon className="size-5" />
                  </div>
                  <CardTitle className="text-base">{item.title}</CardTitle>
                </CardHeader>
                <CardContent>
                  <p className="text-muted-foreground text-sm leading-relaxed">{item.body}</p>
                </CardContent>
              </Card>
            </Reveal>
          )
        })}
      </div>
    </Section>
  )
}
