import type { ReactNode } from 'react'
import { Reveal } from '@/components/Reveal'
import { cn } from '@/lib/utils'

/** A page section: a large faint number, the kicker ("// 01  FEATURES"), a title and a lead, revealed on scroll. */
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
  const number = kicker.match(/\d+/)?.[0]
  return (
    <section id={id} className={cn('relative scroll-mt-16 py-20 sm:py-28', className)}>
      <Reveal className="relative mb-10">
        {number && (
          <span className="font-display text-primary/6 pointer-events-none absolute -top-10 -left-2 text-[7rem] leading-none select-none sm:-top-14 sm:text-[10rem]" aria-hidden="true">
            {number}
          </span>
        )}
        <p className="font-label text-primary/80 relative text-sm font-semibold">{kicker}</p>
        <h2 className="relative mt-2 text-3xl font-bold tracking-tight sm:text-4xl">{title}</h2>
        {lead && <p className="text-muted-foreground relative mt-3 max-w-2xl text-base sm:text-lg">{lead}</p>}
      </Reveal>
      {children}
    </section>
  )
}
