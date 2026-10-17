import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
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

export type Locale = 'en' | 'zh'

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

const messages: Record<Locale, Messages> = {
  en: [eventsDeliveriesEn, mappingsTemplatesEn, rulesDestinationsEn, miscEn].reduce(
    (acc, cur) => deepMerge(acc, cur as Messages),
    mainEn as Messages,
  ),
  zh: [eventsDeliveriesZh, mappingsTemplatesZh, rulesDestinationsZh, miscZh].reduce(
    (acc, cur) => deepMerge(acc, cur as Messages),
    mainZh as Messages,
  ),
}

const STORAGE_KEY = 'zpush-locale'

function resolveKey(obj: unknown, key: string): unknown {
  return key.split('.').reduce<unknown>((acc, part) => {
    if (acc && typeof acc === 'object' && part in (acc as Messages)) {
      return (acc as Messages)[part]
    }
    return undefined
  }, obj)
}

function interpolate(template: string, params?: Record<string, unknown>): string {
  if (!params) return template
  return template.replace(/\{(\w+)\}/g, (m, name) => {
    const v = params[name]
    return v === undefined || v === null ? m : String(v)
  })
}

function translate(locale: Locale, key: string, params?: Record<string, unknown>): string {
  const value = resolveKey(messages[locale], key)
  if (typeof value === 'string') return interpolate(value, params)
  // fallback locale
  const fallback = resolveKey(messages.en, key)
  if (typeof fallback === 'string') return interpolate(fallback, params)
  return key
}

interface I18nContextValue {
  locale: Locale
  setLocale: (locale: Locale) => void
  t: (key: string, paramsOrFallback?: Record<string, unknown> | string) => string
}

const I18nContext = createContext<I18nContextValue | null>(null)

function getInitialLocale(): Locale {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'zh' || saved === 'en') return saved
  } catch {
    // ignore
  }
  return 'zh'
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(getInitialLocale)

  useEffect(() => {
    document.documentElement.setAttribute('lang', locale === 'zh' ? 'zh-CN' : 'en')
  }, [locale])

  const setLocale = useCallback((next: Locale) => {
    setLocaleState(next)
    try {
      localStorage.setItem(STORAGE_KEY, next)
    } catch {
      // ignore
    }
  }, [])

  const t = useCallback(
    (key: string, paramsOrFallback?: Record<string, unknown> | string) => {
      if (typeof paramsOrFallback === 'string') {
        const result = translate(locale, key)
        // vue-i18n behavior: t(key, fallback) returns fallback when key missing
        if (result === key) return paramsOrFallback
        return result
      }
      return translate(locale, key, paramsOrFallback)
    },
    [locale],
  )

  return <I18nContext.Provider value={{ locale, setLocale, t }}>{children}</I18nContext.Provider>
}

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext)
  if (!ctx) throw new Error('useI18n must be used within I18nProvider')
  return ctx
}
