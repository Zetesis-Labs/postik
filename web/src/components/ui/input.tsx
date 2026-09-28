// Adaptado de Postiz v2.24.0: libraries/react-shared-libraries/src/form/input.tsx y
// libraries/react-shared-libraries/src/translation/translated-label.tsx (AGPL-3.0).
import { DetailedHTMLProps, FC, InputHTMLAttributes, ReactNode, useMemo } from 'react';
import { clsx } from 'clsx';
import { useFormContext } from 'react-hook-form';
import { useT } from '@/lib/i18n';

export function TranslatedLabel({
  label,
  translationKey,
  translationParams = {},
  children,
}: {
  label: string;
  translationKey?: string;
  translationParams?: Record<string, string | number>;
  children?: ReactNode;
}) {
  const t = useT();
  const key = translationKey || `label_${label.toLowerCase().replace(/\s+/g, '_').replace(/[^\w]/g, '')}`;
  return (
    <>
      {t(key, label, translationParams)}
      {children}
    </>
  );
}

export const Input: FC<
  DetailedHTMLProps<InputHTMLAttributes<HTMLInputElement>, HTMLInputElement> & {
    removeError?: boolean;
    error?: string;
    disableForm?: boolean;
    label: string;
    name: string;
    icon?: ReactNode;
    translationKey?: string;
    translationParams?: Record<string, string | number>;
  }
> = (props) => {
  const { label, icon, removeError, className, disableForm, error, translationKey, translationParams, ...rest } = props;
  const form = useFormContext();
  const fieldError = form?.formState?.errors?.[props.name]?.message;
  const err = useMemo(() => {
    if (error) return error;
    return typeof fieldError === 'string' ? fieldError : undefined;
  }, [error, fieldError]);
  return (
    <div className="flex flex-col gap-[6px]">
      {!!label && (
        <div className={`text-[14px]`}>
          <TranslatedLabel label={label} translationKey={translationKey} translationParams={translationParams} />
        </div>
      )}
      <div
        className={clsx(
          'bg-newBgColorInner h-[42px] border-newTableBorder border rounded-[8px] text-textColor placeholder-textColor flex items-center justify-center',
          className
        )}
      >
        {icon && <div className="ps-[16px]">{icon}</div>}
        <input
          className={clsx(
            'h-full bg-transparent outline-none flex-1 text-[14px] text-textColor',
            icon ? 'pl-[8px] pe-[16px]' : 'px-[16px]'
          )}
          {...(disableForm ? {} : form.register(props.name))}
          {...rest}
        />
      </div>
      {!removeError && <div className="text-red-400 text-[12px]">{err || <>&nbsp;</>}</div>}
    </div>
  );
};
