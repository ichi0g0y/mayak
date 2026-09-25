// The languages the page comes in. The browser's preferred language picks
// the default (state.ts); the header's dropdown changes it.
export const languages = [
  { code: 'ja', label: '日本語', html: 'ja' },
  { code: 'en', label: 'English', html: 'en' },
  { code: 'ru', label: 'Русский', html: 'ru' },
  { code: 'de', label: 'Deutsch', html: 'de' },
  { code: 'zh', label: '简体中文', html: 'zh-CN' },
] as const

export type Lang = (typeof languages)[number]['code']

export function isLang(value: unknown): value is Lang {
  return languages.some((language) => language.code === value)
}

/** The language for an <html lang> value. */
export function htmlLang(lang: Lang): string {
  return languages.find((language) => language.code === lang)?.html ?? lang
}

/** The first of the browser's preferred languages the page has, else English. */
export function detectLang(preferred: readonly string[]): Lang {
  for (const tag of preferred) {
    const code = tag.toLowerCase().split('-')[0]
    if (isLang(code)) return code
  }
  return 'en'
}
