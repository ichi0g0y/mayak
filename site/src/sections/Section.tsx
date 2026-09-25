import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

/** A page section with the tactical kicker ("// 01  FEATURES"), a title and a lead. */
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
    <section id={id} className={cn('scroll-mt-20 py-16 sm:py-24', className)}>
      <div className="mb-10">
        <p className="font-label text-primary/80 text-sm font-semibold">{kicker}</p>
        <h2 className="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">{title}</h2>
        {lead && <p className="text-muted-foreground mt-3 max-w-2xl text-base sm:text-lg">{lead}</p>}
      </div>
      {children}
    </section>
  )
}
