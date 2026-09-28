import i18next from 'i18next';
import { initReactI18next, useTranslation } from 'react-i18next';
import en from '@/locales/en.json';
import es from '@/locales/es.json';
import postikEn from '@/locales/postik.en.json';
import postikEs from '@/locales/postik.es.json';

export const languages = ['es', 'en'] as const;
export type Language = (typeof languages)[number];

const storageKey = 'postik_language';

export function detectLanguage(stored: string | null, browser: readonly string[]): Language {
  if (stored === 'es' || stored === 'en') {
    return stored;
  }
  for (const tag of browser) {
    const base = tag.toLowerCase().split('-')[0];
    if (base === 'es' || base === 'en') {
      return base;
    }
  }
  return 'en';
}

function readStoredLanguage(): string | null {
  try {
    return window.localStorage.getItem(storageKey);
  } catch {
    return null;
  }
}

export function initI18n() {
  const lng = detectLanguage(readStoredLanguage(), navigator.languages ?? [navigator.language]);
  document.documentElement.lang = lng;
  return i18next.use(initReactI18next).init({
    resources: {
      en: { translation: { ...en, ...postikEn } },
      es: { translation: { ...es, ...postikEs } },
    },
    lng,
    fallbackLng: 'en',
    interpolation: { escapeValue: false },
  });
}

export function changeLanguage(language: Language) {
  try {
    window.localStorage.setItem(storageKey, language);
  } catch {
    // Without storage the choice lasts until the page reloads.
  }
  document.documentElement.lang = language;
  return i18next.changeLanguage(language);
}

export function currentLanguage(): Language {
  return i18next.resolvedLanguage === 'es' ? 'es' : 'en';
}

export function useT() {
  return useTranslation().t;
}
