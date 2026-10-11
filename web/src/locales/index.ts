import { createI18n } from 'vue-i18n'
import en from './en.json'
import zh from './zh.json'

const savedLocale = localStorage.getItem('zpush-locale') || 'zh'

const i18n = createI18n({
  legacy: false,
  locale: savedLocale,
  fallbackLocale: 'en',
  messages: { en, zh },
})

export default i18n

export type Locales = 'en' | 'zh'

export function setLocale(locale: Locales) {
  i18n.global.locale.value = locale
  localStorage.setItem('zpush-locale', locale)
  document.documentElement.setAttribute('lang', locale === 'zh' ? 'zh-CN' : 'en')
}

export function getLocale(): Locales {
  return i18n.global.locale.value as Locales
}
