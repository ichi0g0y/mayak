// Page state (jotai): the display language and the latest GitHub release.
import { atom } from 'jotai'
import { atomWithStorage } from 'jotai/utils'
import { detectLang, isLang, type Lang } from './i18n/languages'

const LANG_KEY = 'mayak-lang'

function initialLang(): Lang {
  try {
    const saved = localStorage.getItem(LANG_KEY)
    if (isLang(saved)) return saved
  } catch {
    /* private mode */
  }
  return detectLang(navigator.languages?.length ? navigator.languages : [navigator.language])
}

/** The display language: the browser's preferred one at first, then whatever the visitor picked. */
export const langAtom = atomWithStorage<Lang>(LANG_KEY, initialLang(), undefined, { getOnInit: true })

export const REPOSITORY = 'ichi0g0y/mayak'
export const RELEASES_URL = `https://github.com/${REPOSITORY}/releases`

export type Asset = { name: string; url: string; size: number }
export type Release = { tag: string; version: string; name: string; url: string; publishedAt: string; assets: Asset[] }

/** The installer and the archives the release workflow publishes. */
export const ARCHIVES = {
  windowsInstaller: 'Mayak-Setup-windows-amd64.exe',
  windows: 'Mayak-windows-amd64.zip',
  macArm: 'Mayak-darwin-arm64.tar.gz',
  macIntel: 'Mayak-darwin-amd64.tar.gz',
  linux: 'Mayak-linux-amd64.tar.gz',
  checksums: 'SHA256SUMS.txt',
} as const

async function fetchLatest(): Promise<Release> {
  const response = await fetch(`https://api.github.com/repos/${REPOSITORY}/releases/latest`, {
    headers: { Accept: 'application/vnd.github+json' },
  })
  if (!response.ok) throw new Error(`GitHub: ${response.status}`)
  const data = (await response.json()) as {
    tag_name: string
    name: string
    html_url: string
    published_at: string
    assets: { name: string; browser_download_url: string; size: number }[]
  }
  return {
    tag: data.tag_name,
    version: data.tag_name.replace(/^v/, ''),
    name: data.name,
    url: data.html_url,
    publishedAt: data.published_at,
    assets: data.assets.map((a) => ({ name: a.name, url: a.browser_download_url, size: a.size })),
  }
}

export type ReleaseState = { state: 'loading' } | { state: 'hasData'; data: Release } | { state: 'hasError'; error: unknown }

/** The latest release, fetched once the first component reads it; the page never suspends on it. */
export const releaseLoadableAtom = atom<ReleaseState>({ state: 'loading' })
releaseLoadableAtom.onMount = (set) => {
  fetchLatest()
    .then((data) => set({ state: 'hasData', data }))
    .catch((error: unknown) => set({ state: 'hasError', error }))
}

/** The release asset of that name, when the release has loaded. */
export function findAsset(release: Release | undefined, name: string): Asset | undefined {
  return release?.assets.find((a) => a.name === name)
}

export function formatSize(bytes: number): string {
  return `${(bytes / 1048576).toFixed(0)} MB`
}

export type Platform = 'windows' | 'mac' | 'linux'

/** The visitor's OS, to put its download first. */
export const platformAtom = atom<Platform>(() => {
  const ua = navigator.userAgent
  if (/Mac|iPhone|iPad/.test(ua)) return 'mac'
  if (/Linux|X11/.test(ua) && !/Android/.test(ua)) return 'linux'
  return 'windows'
})
