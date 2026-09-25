import { useEffect, useRef, type ReactNode } from 'react'
import { cn } from '@/lib/utils'

/** Fades and slides its children in when they scroll into view. */
export function Reveal({ children, className, delay = 0, as: Tag = 'div' }: { children: ReactNode; className?: string; delay?: number; as?: 'div' | 'li' | 'section' }) {
  const ref = useRef<HTMLElement>(null)
  useEffect(() => {
    const node = ref.current
    if (!node) return
    if (!('IntersectionObserver' in window)) {
      node.classList.add('is-visible')
      return
    }
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            node.classList.add('is-visible')
            observer.disconnect()
          }
        }
      },
      { rootMargin: '0px 0px -10% 0px', threshold: 0.1 },
    )
    observer.observe(node)
    return () => observer.disconnect()
  }, [])
  const Component = Tag as 'div'
  return (
    <Component ref={ref as never} className={cn('reveal', className)} style={{ transitionDelay: `${delay}ms` }}>
      {children}
    </Component>
  )
}
