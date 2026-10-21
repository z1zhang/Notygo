import { useEffect, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import i18next from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import mainEn from './locales/en.json'
import mainZh from './locales/zh.json'
import eventsDeliveriesEn from './locales/extra/events-deliveries.en.json'
import eventsDeliveriesZh from './locales/extra/events-deliveries.zh.json'
import mappingsTemplatesEn from './locales/extra/mappings-templates.en.json'
import mappingsTemplatesZh from './locales/extra/mappings-templates.zh.json'
import rulesDestinationsEn from './locales/extra/rules-destinations.en.json'
import rulesDestinationsZh from './locales/extra/rules-destinations.zh.json'
import miscEn from './locales/extra/misc.en.json'
import miscZh from './locales/extra/misc.zh.json'

export type Locale = 'zh' | 'en'

type Messages = Record<string, unknown>

function deepMerge(base: Messages, override: Messages): Messages {
  const result: Messages = { ...base }
  for (const [key, value] of Object.entries(override)) {
    const existing = result[key]
    if (value && typeof value === 'object' && !Array.isArray(value) &&
        existing && typeof existing === 'object' && !Array.isArray(existing)) {
      result[key] = deepMerge(existing as Messages, value as Messages)
    } else {
      result[key] = value
    }
  }
  return result
}

const zhMessages = [eventsDeliveriesZh, mappingsTemplatesZh, rulesDestinationsZh, miscZh].reduce(
  (acc, cur) => deepMerge(acc, cur as Messages),
  mainZh as Messages,
)
const enMessages = [eventsDeliveriesEn, mappingsTemplatesEn, rulesDestinationsEn, miscEn].reduce(
  (acc, cur) => deepMerge(acc, cur as Messages),
  mainEn as Messages,
)

/** `{ts, relative}` → "3 分钟前" / "3 minutes ago". */
function formatRelative(value: number, locale: string): string {
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  const seconds = (value - Date.now()) / 1000
  const abs = Math.abs(seconds)
  if (abs < 60) return rtf.format(Math.round(seconds), 'second')
  if (abs < 3600) return rtf.format(Math.round(seconds / 60), 'minute')
  if (abs < 86400) return rtf.format(Math.round(seconds / 3600), 'hour')
  return rtf.format(Math.round(seconds / 86400), 'day')
}

void i18next
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    supportedLngs: ['zh', 'en'],
    fallbackLng: 'en',
    detection: {
      // Previously: localStorage only, default zh. Now: browser language on
      // first visit, persisted under the same zpush-locale key.
      order: ['localStorage', 'navigator'],
      lookupLocalStorage: 'zpush-locale',
      caches: ['localStorage'],
    },
    resources: {
      zh: { translation: zhMessages },
      en: { translation: enMessages },
    },
    // Locales were written for single-brace {name} interpolation; React does
    // its own escaping, so i18next's HTML escaping must stay off.
    interpolation: {
      prefix: '{',
      suffix: '}',
      escapeValue: false,
    },
    react: { useSuspense: false },
  })

// Built-in i18next v26 formatters cover {v, number}, {v, datetime(dateStyle: medium)},
// {v, currency}, {v, list}… "X ago from a timestamp" needs the offset computed,
// so register one custom formatter via the library's extension point.
void (i18next.services?.formatter as { add: (name: string, fc: (lng: string) => (v: number) => string) => void } | undefined)
  ?.add('relative', (lng) => (value) => formatRelative(Number(value), lng === 'en' ? 'en' : 'zh-CN'))

// ── i18next TypeScript augmentation (the library's own key typing) ─────────
// zh.json is the source of truth for the key set.

/** zh is the source of truth for the key set. */
type ZhMessages = typeof mainZh &
  typeof eventsDeliveriesZh &
  typeof mappingsTemplatesZh &
  typeof rulesDestinationsZh &
  typeof miscZh
type EnMessages = typeof mainEn &
  typeof eventsDeliveriesEn &
  typeof mappingsTemplatesEn &
  typeof rulesDestinationsEn &
  typeof miscEn

declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'translation'
    resources: { translation: ZhMessages }
  }
}

type Primitive = string | number | boolean
/** All dotted key paths of a nested locale tree. */
type FlattenKeys<T, Prefix extends string = ''> = T extends Primitive
  ? Prefix
  : { [K in keyof T & string]: FlattenKeys<T[K], Prefix extends '' ? K : `${Prefix}.${K}`> }[keyof T & string]

/** Cast helper for genuinely dynamic keys (enum values from the API). */
export type TranslationKey = FlattenKeys<ZhMessages>

type EnMissingKeys = Exclude<TranslationKey, FlattenKeys<EnMessages>>
// Errors here ⇒ en.json (or an extra/*.en.json) is missing a zh key.
const _enKeyParity: EnMissingKeys extends never ? true : never = true
void _enKeyParity

/** Keeps <html lang> in sync; react-i18next itself needs no provider. */
export function I18nProvider({ children }: { children: ReactNode }) {
  const { i18n } = useTranslation()
  useEffect(() => {
    i18n.on('languageChanged', sync)
    return () => i18n.off('languageChanged', sync)
    function sync(lng: string) {
      document.documentElement.setAttribute('lang', lng === 'en' ? 'en' : 'zh-CN')
    }
  }, [i18n])
  return <I18nextProvider i18n={i18next}>{children}</I18nextProvider>
}

export function useLocale(): { locale: Locale; setLocale: (locale: Locale) => void } {
  const { i18n } = useTranslation()
  return {
    locale: (i18n.language === 'en' ? 'en' : 'zh') as Locale,
    setLocale: (next: Locale) => {
      void i18n.changeLanguage(next) // the detector persists it to localStorage
    },
  }
}
