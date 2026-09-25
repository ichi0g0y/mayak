import { useMemo } from 'react'
import { useAtomValue } from 'jotai'
import { marked } from 'marked'
import ja from '../../../CHANGELOG.md?raw'
import en from '../../../CHANGELOG.en.md?raw'
import { useT } from '@/i18n'
import { langAtom } from '@/state'
import { Section } from './Section'

// The changelog lives in the repository as Markdown (CHANGELOG.md in Japanese,
// CHANGELOG.en.md in English) and is rendered here as is; the other page
// languages show the English one. Each version heading gets its version as
// its id (v0.1.8), so the app's update notice can link to #v0.1.8.
marked.use({
  renderer: {
    heading({ tokens, depth }) {
      const text = this.parser.parseInline(tokens)
      const first = tokens.map((token) => token.raw).join('').trim().split(/\s+/)[0] ?? ''
      const id = /^v\d/.test(first) ? first : 'unreleased'
      return `<h${depth} id="${id}">${text}</h${depth}>\n`
    },
  },
})

/** The entries only: everything from the first version heading on. */
function entries(markdown: string): string {
  const start = markdown.indexOf('\n## ')
  return start < 0 ? markdown : markdown.slice(start + 1)
}

export function Changelog() {
  const t = useT()
  const lang = useAtomValue(langAtom)
  const html = useMemo(() => marked.parse(entries(lang === 'ja' ? ja : en), { async: false }) as string, [lang])
  return (
    <Section id="changelog" kicker={t.changelog.kicker} title={t.changelog.title} lead={t.changelog.lead}>
      <div className="changelog max-w-3xl" dangerouslySetInnerHTML={{ __html: html }} />
    </Section>
  )
}
