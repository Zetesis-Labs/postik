// Adaptado de Postiz v2.24.0: libraries/react-shared-libraries/src/form/slider.tsx (AGPL-3.0).
import { FC } from 'react';
import clsx from 'clsx';

export const Slider: FC<{ value: 'on' | 'off'; fill?: boolean; onChange: (value: 'on' | 'off') => void; label?: string }> = ({
  value,
  fill,
  onChange,
  label,
}) => (
  <div
    role="switch"
    aria-checked={value === 'on'}
    aria-label={label}
    tabIndex={0}
    className={clsx('w-[57px] h-[34px] p-[4px] border-fifth border rounded-[100px]', value === 'on' && fill && 'bg-customColor4')}
    onClick={() => onChange(value === 'on' ? 'off' : 'on')}
    onKeyDown={(e) => {
      if (e.key === ' ' || e.key === 'Enter') {
        e.preventDefault();
        onChange(value === 'on' ? 'off' : 'on');
      }
    }}
  >
    <div className="w-full h-full relative rounded-[100px]">
      <div
        className={clsx(
          'absolute left-0 top-0 w-[24px] h-[24px] bg-customColor5 rounded-full transition-all cursor-pointer',
          value === 'on' ? 'left-[100%] -translate-x-[100%]' : 'left-0'
        )}
      />
    </div>
  </div>
);
