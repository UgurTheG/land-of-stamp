import { createContext } from 'react';
import type { Locale } from '../i18n/messages';
import { messages } from '../i18n/messages';

export interface LocaleContextType {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  m: (typeof messages)[Locale];
}

export const LocaleContext = createContext<LocaleContextType | null>(null);

