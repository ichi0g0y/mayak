import { useT } from '@/i18n'
import { RELEASES_URL, REPOSITORY } from '@/state'

export function Footer() {
  const t = useT()
  const columns: { title: string; links: [string, string][] }[] = [
    {
      title: t.footer.product,
      links: [
        ['/#download', t.footer.download],
        [RELEASES_URL, t.footer.releases],
        ['/license', t.footer.licensePage],
      ],
    },
    {
      title: t.footer.community,
      links: [
        [`https://github.com/${REPOSITORY}`, t.footer.source],
        [`https://github.com/${REPOSITORY}/issues`, t.footer.issues],
        ['https://tarkov.dev/', t.footer.tarkovDev],
        ['https://tarkovtracker.org/', t.footer.tracker],
      ],
    },
  ]
  return (
    <footer>
      <div className="mx-auto grid max-w-6xl gap-10 border-x px-6 py-12 sm:px-10 md:grid-cols-[1.4fr_1fr_1fr]">
        <div>
          <div className="flex items-center gap-2.5">
            <img src="/assets/mayak-mark.png" alt="" width={24} height={24} className="size-6" />
            <span className="font-extrabold tracking-tight">MAYAK</span>
          </div>
          <p className="text-muted-foreground mt-3 max-w-sm text-sm">{t.footer.tagline}</p>
          <p className="text-muted-foreground mt-6 max-w-md text-xs leading-relaxed">{t.footer.credit}</p>
          <p className="text-muted-foreground mt-3 font-mono text-xs">© {new Date().getFullYear()} MAYAK contributors · GPL-3.0</p>
        </div>
        {columns.map((column) => (
          <div key={column.title}>
            <p className="eyebrow">{column.title}</p>
            <ul className="mt-4 grid gap-2 text-sm">
              {column.links.map(([href, label]) => (
                <li key={href}>
                  <a href={href} rel={href.startsWith('http') ? 'noopener' : undefined} className="text-muted-foreground hover:text-foreground transition-colors">
                    {label}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </footer>
  )
}
