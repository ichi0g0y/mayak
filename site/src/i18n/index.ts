// Every string on the page, per language. useT() returns the current
// language's set (state.ts holds the choice).
import { useAtomValue } from 'jotai'
import { langAtom } from '../state'
import { ja, type Messages } from './ja'
import { en } from './en'
import { ru } from './ru'
import { de } from './de'
import { zh } from './zh'
import type { Lang } from './languages'

export { languages, htmlLang, detectLang, isLang } from './languages'
export type { Lang, Messages }

export const messages: Record<Lang, Messages> = { ja, en, ru, de, zh }

export function useT(): Messages {
  return messages[useAtomValue(langAtom)]
}
