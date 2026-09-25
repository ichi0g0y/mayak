import type { Lang } from '@/i18n/languages'

// Screenshots of the app live under public/screenshots/<lang>/ for the
// languages the app itself speaks (Japanese and English); every other
// page language shows the English set. Captures that do not depend on the
// app's language (the installer, SmartScreen, EFT's own settings) sit in
// public/screenshots/ and are shared.

export function screenshotLang(lang: Lang): 'ja' | 'en' {
  return lang === 'ja' ? 'ja' : 'en'
}

/** The path of a localized screenshot for the page language. */
export function localized(lang: Lang, file: string): string {
  return `/screenshots/${screenshotLang(lang)}/${file}`
}

/** The path of a screenshot that is the same in every language. */
export function shared(file: string): string {
  return `/screenshots/${file}`
}
