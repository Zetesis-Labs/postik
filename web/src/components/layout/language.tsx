// Adaptado de Postiz v2.24.0: apps/frontend/src/components/layout/language.component.tsx (AGPL-3.0).
import { useCallback } from 'react';
import ReactCountryFlag from 'react-country-flag';
import clsx from 'clsx';
import { useModals } from '@/components/ui/modals';
import { changeLanguage, currentLanguage, languages, Language, useT } from '@/lib/i18n';

const flagFor: Record<Language, string> = { es: 'ES', en: 'GB' };

export const ChangeLanguageComponent = () => {
  const current = currentLanguage();
  const modals = useModals();

  const handleLanguageChange = (language: Language) => {
    changeLanguage(language);
    modals.closeCurrent();
  };

  const getLanguageName = useCallback((code: string) => {
    try {
      return new Intl.DisplayNames([code], { type: 'language' }).of(code);
    } catch {
      return code;
    }
  }, []);

  return (
    <div className="relative">
      <div className="grid grid-cols-4 gap-2">
        {languages.map((language) => (
          <button
            type="button"
            className={clsx(
              'flex items-center flex-col bg-newTableHeader hover:bg-newTableBorder p-[20px] cursor-pointer gap-2',
              language === current ? 'border border-textColor' : ''
            )}
            key={language}
            data-language={language}
            onClick={() => handleLanguageChange(language)}
          >
            <ReactCountryFlag countryCode={flagFor[language]} svg style={{ width: '1.5em', height: '1.5em' }} title={language} />
            <div className={language === current ? 'font-bold' : 'font-normal'}>{getLanguageName(language)}</div>
          </button>
        ))}
      </div>
    </div>
  );
};

export const LanguageComponent = () => {
  const modal = useModals();
  const t = useT();
  const current = currentLanguage();
  const openModal = () => {
    modal.openModal({
      title: t('change_language', 'Change Language'),
      withCloseButton: true,
      children: <ChangeLanguageComponent />,
    });
  };
  return (
    <button
      type="button"
      onClick={openModal}
      aria-label={t('change_language', 'Change Language')}
      className="rounded-full overflow-hidden h-[22px] w-[22px] relative cursor-pointer"
    >
      <ReactCountryFlag
        countryCode={flagFor[current]}
        svg
        style={{
          width: '22px',
          height: '22px',
          position: 'absolute',
          left: '50%',
          top: '50%',
          transform: 'translate(-50%, -50%)',
          objectFit: 'cover',
        }}
        title={current}
      />
    </button>
  );
};
