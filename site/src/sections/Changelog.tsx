import { useEffect, useMemo, useRef } from 'react'
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
      // Sections (## v0.1.17, ## nightly) get an id; the kinds of change
      // under them (### New, ### Improved, ### Fixed) do not.
      if (depth !== 2) return `<h${depth}>${text}</h${depth}>\n`
      const id = /^v\d/.test(first) ? first : first.toLowerCase() === 'nightly' ? 'nightly' : 'unreleased'
      return `<h${depth} id="${id}">${text}</h${depth}>\n`
    },
  },
})

/**
 * The entries: everything from the first section heading on — the nightly
 * section (changes not released yet, with the note under its heading) and
 * then the released versions (## v0.1.17 …).
 */
function entries(markdown: string): string {
  const start = markdown.indexOf('\n## ')
  return start < 0 ? markdown : markdown.slice(start + 1)
}

/**
 * The page's HTML: the nightly section folded (its heading is the summary;
 * a link to #nightly opens it, see Changelog), then the released versions.
 */
function render(markdown: string): string {
  const all = entries(markdown)
  const released = all.search(/(^|\n)## v\d/)
  if (released <= 0) return marked.parse(all, { async: false }) as string
  const nightly = all.slice(0, released)
  const heading = nightly.match(/^## (.+)$/m)?.[1] ?? 'nightly'
  // The note under the heading (what the section holds) shows while folded.
  const note = nightly.match(/^## .+\n+([^\n-][^\n]*)/)?.[1] ?? ''
  const body = marked.parse(nightly.replace(/^## .+$/m, '').replace(note, ''), { async: false }) as string
  const rest = marked.parse(all.slice(released).replace(/^\n/, ''), { async: false }) as string
  const summary = `<h2>${marked.parseInline(heading)}</h2>${note ? `<small>${marked.parseInline(note)}</small>` : ''}`
  return `<details class="changelog-nightly" id="nightly"><summary>${summary}</summary>${body}</details>${rest}`
}

export function Changelog() {
  const t = useT()
  const lang = useAtomValue(langAtom)
  const html = useMemo(() => render(lang === 'ja' ? ja : en), [lang])
  const box = useRef<HTMLDivElement>(null)
  // A link to #nightly opens the folded nightly section.
  useEffect(() => {
    const open = () => {
      if (location.hash !== '#nightly') return
      const details = box.current?.querySelector<HTMLDetailsElement>('#nightly')
      if (details) details.open = true
    }
    open()
    addEventListener('hashchange', open)
    return () => removeEventListener('hashchange', open)
  }, [html])
  return (
    <Section id="changelog" kicker={t.changelog.kicker} title={t.changelog.title} lead={t.changelog.lead}>
      <div ref={box} className="changelog max-w-3xl" dangerouslySetInnerHTML={{ __html: html }} />
    </Section>
  )
}
