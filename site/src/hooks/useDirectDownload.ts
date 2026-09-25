import { atom, useAtomValue } from 'jotai'
import { ARCHIVES, findAsset, platformAtom, RELEASES_URL, releaseLoadableAtom, type Asset, type Platform } from '@/state'

/** Which Mac build the visitor wants; Apple Silicon unless they say Intel. */
export const macArchAtom = atom<'arm64' | 'amd64'>('arm64')

/** The release asset for a platform (and, on a Mac, the chosen CPU). */
export function assetFor(platform: Platform, macArch: 'arm64' | 'amd64') {
  switch (platform) {
    case 'windows':
      return ARCHIVES.windowsInstaller
    case 'mac':
      return macArch === 'arm64' ? ARCHIVES.macArm : ARCHIVES.macIntel
    case 'linux':
      return ARCHIVES.linux
  }
}

/**
 * The link every "download" control points at: the file built for the
 * visitor's OS, straight from the release (no detour through GitHub's
 * pages), or the releases page while the release is still unknown.
 */
export function useDirectDownload(): { href: string; asset?: Asset; download?: string; platform: Platform } {
  const release = useAtomValue(releaseLoadableAtom)
  const platform = useAtomValue(platformAtom)
  const macArch = useAtomValue(macArchAtom)
  const latest = release.state === 'hasData' ? release.data : undefined
  const asset = findAsset(latest, assetFor(platform, macArch))
  if (asset) return { href: asset.url, asset, download: asset.name, platform }
  return { href: `${RELEASES_URL}/latest`, platform }
}
