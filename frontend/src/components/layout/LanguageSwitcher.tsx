import { Languages } from 'lucide-react';
import { useLocale } from '../../hooks/useLocale';
import type { Locale } from '../../i18n/messages';

interface Props {
  fullWidth?: boolean;
}

export default function LanguageSwitcher({ fullWidth = false }: Props) {
  const { locale, setLocale, m } = useLocale();

  const options: { code: Locale; label: string }[] = [
    { code: 'en', label: 'EN' },
    { code: 'de', label: 'DE' },
  ];

  return (
    <div
      className={`flex items-center gap-1 rounded-xl border border-white/10 bg-white/5 p-1 ${
        fullWidth ? 'w-full justify-between' : ''
      }`}
      aria-label={m.common.language}
    >
      <div className="flex items-center gap-2 px-2 text-indigo-200 text-sm">
        <Languages className="w-4 h-4" />
        {fullWidth && <span>{m.common.language}</span>}
      </div>
      <div className="flex items-center gap-1">
        {options.map((option) => {
          const active = locale === option.code;
          return (
            <button
              key={option.code}
              type="button"
              onClick={() => setLocale(option.code)}
              aria-pressed={active}
              title={option.code === 'en' ? m.common.english : m.common.german}
              className={`rounded-lg px-2.5 py-1.5 text-xs font-semibold transition-all cursor-pointer ${
                active
                  ? 'bg-primary text-white shadow-lg shadow-primary/20'
                  : 'text-indigo-200 hover:bg-white/10 hover:text-white'
              }`}
            >
              {option.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

