import { useAtomValue } from 'jotai'
import { ARCHIVES, findAsset, platformAtom, RELEASES_URL, releaseLoadableAtom, type Asset } from '@/state'

/**
 * The link a "download" control points at: the Windows installer itself
 * for Windows visitors (the release's own file, no detour through GitHub),
 * the download section for macOS and Linux, where the CPU has to be
 * chosen, and the releases page while the release is still unknown.
 */
export function useDirectDownload(): { href: string; asset?: Asset; download?: string } {
  const release = useAtomValue(releaseLoadableAtom)
  const platform = useAtomValue(platformAtom)
  const latest = release.state === 'hasData' ? release.data : undefined
  if (platform !== 'windows') return { href: '#download' }
  const installer = findAsset(latest, ARCHIVES.windowsInstaller)
  if (installer) return { href: installer.url, asset: installer, download: installer.name }
  return { href: latest ? '#download' : `${RELEASES_URL}/latest` }
}
