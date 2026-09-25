import type { ReactNode } from 'react'
import { Reveal } from '@/components/Reveal'
import { cn } from '@/lib/utils'

/** A page section: a small upper-case label with a rule, a title and a lead, revealed on scroll. */
export function Section({
  id,
  kicker,
  title,
  lead,
  children,
  className,
}: {
  id: string
  kicker: string
  title: string
  lead?: string
  children: ReactNode
  className?: string
}) {
  return (
    <section id={id} className={cn('scroll-mt-16 py-16 sm:py-24', className)}>
      <Reveal className="mb-8 sm:mb-10">
        <p className="font-label rule-after text-primary/70 text-sm">{kicker}</p>
        <h2 className="mt-3 text-2xl font-bold tracking-tight sm:text-3xl">{title}</h2>
        {lead && <p className="text-muted-foreground mt-3 max-w-2xl text-base">{lead}</p>}
      </Reveal>
      {children}
    </section>
  )
}
