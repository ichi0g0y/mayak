import type { ReactNode } from 'react'
import { Reveal } from '@/components/Reveal'
import { cn } from '@/lib/utils'

/** A page section: an eyebrow label, a large headline and a lead, revealed on scroll. */
export function Section({
  id,
  kicker,
  title,
  lead,
  children,
  className,
  aside,
}: {
  id: string
  kicker: string
  title: string
  lead?: string
  children: ReactNode
  className?: string
  /** Shown to the right of the heading on wide screens. */
  aside?: ReactNode
}) {
  const lines = title.split('\n')
  return (
    <section id={id} className={cn('scroll-mt-14 border-b px-6 py-16 sm:px-10 sm:py-24', className)}>
      <Reveal className={cn('mb-10 sm:mb-12', aside && 'grid gap-8 lg:grid-cols-[1.2fr_1fr] lg:items-end')}>
        <div>
          <p className="eyebrow">{kicker}</p>
          <h2 className="headline mt-4 text-4xl sm:text-5xl">
            {lines.map((line, i) => (
              <span key={i} className="block">
                {line}
              </span>
            ))}
          </h2>
          {lead && <p className="text-muted-foreground mt-5 max-w-xl text-base sm:text-lg">{lead}</p>}
        </div>
        {aside}
      </Reveal>
      {children}
    </section>
  )
}
