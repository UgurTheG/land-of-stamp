import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { messages, supportedLocales, type Locale } from '../i18n/messages';
import { LocaleContext } from './localeContext';

const LOCALE_STORAGE_KEY = 'land_of_stamp_locale';


function getInitialLocale(): Locale {
  if (typeof window === 'undefined') return 'en';

  const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY);
  if (stored && supportedLocales.includes(stored as Locale)) {
    return stored as Locale;
  }

  const browserLocale = window.navigator.language.toLowerCase();
  if (browserLocale.startsWith('de')) return 'de';
  return 'en';
}

export function LocaleProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<Locale>(getInitialLocale);

  useEffect(() => {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);
    document.documentElement.lang = locale;
  }, [locale]);

  const value = useMemo(
    () => ({ locale, setLocale, m: messages[locale] }),
    [locale],
  );

  return <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>;
}

