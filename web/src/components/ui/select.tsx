// Adaptado de Postiz v2.24.0: libraries/react-shared-libraries/src/form/select.tsx (AGPL-3.0).
import { DetailedHTMLProps, forwardRef, SelectHTMLAttributes, useMemo } from 'react';
import { clsx } from 'clsx';
import { useFormContext } from 'react-hook-form';
import { TranslatedLabel } from '@/components/ui/input';

type SelectProps = DetailedHTMLProps<SelectHTMLAttributes<HTMLSelectElement>, HTMLSelectElement> & {
  error?: string;
  disableForm?: boolean;
  label: string;
  name: string;
  hideErrors?: boolean;
  translationKey?: string;
};

export const Select = forwardRef<HTMLSelectElement, SelectProps>((props, ref) => {
  const { label, className, hideErrors, disableForm, error, translationKey, ...rest } = props;
  const form = useFormContext();
  const registration = disableForm ? undefined : form.register(props.name);
  const fieldError = form?.formState?.errors?.[props.name]?.message;
  const err = useMemo(() => error || (typeof fieldError === 'string' ? fieldError : undefined), [error, fieldError]);
  return (
    <div className={clsx('flex flex-col', label ? 'gap-[6px]' : '')}>
      <div className={`text-[14px]`}>
        <TranslatedLabel label={label} translationKey={translationKey} />
      </div>
      <select
        {...registration}
        ref={registration ? registration.ref : ref}
        className={clsx(
          'h-[42px] bg-newBgColorInner px-[16px] outline-none border-newTableBorder border rounded-[8px] text-[14px]',
          className
        )}
        {...rest}
      />
      {!hideErrors && <div className="text-red-400 text-[12px]">{err || <>&nbsp;</>}</div>}
    </div>
  );
});
