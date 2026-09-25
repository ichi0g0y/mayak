import { useT } from '@/i18n'
import { REPOSITORY } from '@/state'

export function Footer() {
  const t = useT()
  return (
    <footer className="border-t">
      <div className="text-muted-foreground mx-auto max-w-6xl px-4 py-10 text-xs leading-relaxed sm:px-6">
        <div className="mb-4 flex items-center gap-2.5">
          <img src="/assets/mayak-mark.png" alt="" width={24} height={24} className="size-6" />
          <span className="font-display text-primary tracking-[0.2em]">MAYAK</span>
        </div>
        <p className="max-w-3xl">{t.footer.credit}</p>
        <p className="mt-3 flex flex-wrap gap-4">
          <a href={`https://github.com/${REPOSITORY}`} rel="noopener" className="hover:text-foreground">
            {t.footer.source}
          </a>
          <a href={`https://github.com/${REPOSITORY}/issues`} rel="noopener" className="hover:text-foreground">
            {t.footer.issues}
          </a>
          <a href="/license" className="hover:text-foreground">
            {t.footer.licensePage}
          </a>
          <a href="https://tarkov.dev/" rel="noopener" className="hover:text-foreground">
            tarkov.dev
          </a>
          <a href="https://tarkovtracker.org/" rel="noopener" className="hover:text-foreground">
            TarkovTracker
          </a>
        </p>
      </div>
    </footer>
  )
}
