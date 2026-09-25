import { Reveal } from '@/components/Reveal'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { useT } from '@/i18n'
import { Section } from './Section'

export function Faq() {
  const t = useT()
  return (
    <Section id="faq" kicker={t.faq.kicker} title={t.faq.title}>
      <Reveal>
        <Accordion type="single" collapsible className="bg-card/80 rounded-xl border px-5">
          {t.faq.items.map((item, i) => (
            <AccordionItem key={item.q} value={`q${i}`}>
              <AccordionTrigger className="text-base">{item.q}</AccordionTrigger>
              <AccordionContent className="text-muted-foreground leading-relaxed">{item.a}</AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      </Reveal>
    </Section>
  )
}
