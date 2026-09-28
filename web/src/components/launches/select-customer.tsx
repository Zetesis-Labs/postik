// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/select.customer.tsx (AGPL-3.0).
import { FC, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import clsx from 'clsx';
import { useT } from '@/lib/i18n';
import { useToaster } from '@/components/ui/toaster';
import { DropdownArrowIcon, UserIcon } from '@/components/ui/icons';
import { Integrations } from '@/components/launches/calendar-context';

export function useClickOutside<T extends HTMLElement>(onOutside: () => void) {
  const ref = useRef<T>(null);
  useEffect(() => {
    const listener = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) {
        onOutside();
      }
    };
    document.addEventListener('mousedown', listener);
    return () => document.removeEventListener('mousedown', listener);
  }, [onOutside]);
  return ref;
}

export const SelectCustomer: FC<{
  onChange: (value: string) => void;
  integrations: Integrations[];
  customer?: string;
  onSelected?: () => void;
}> = ({ onChange, integrations, onSelected }) => {
  const toaster = useToaster();
  const t = useT();
  const [pos, setPos] = useState<{ top?: number; left?: number }>({});
  const [open, setOpen] = useState(false);
  const close = useCallback(() => setOpen(false), []);
  const ref = useClickOutside<HTMLDivElement>(close);

  const openClose = useCallback(() => {
    if (open) {
      setOpen(false);
      return;
    }
    const box = ref.current?.getBoundingClientRect();
    setPos({ top: (box?.y ?? 0) + (box?.height ?? 0), left: box?.x ?? 0 });
    setOpen(true);
  }, [open, ref]);

  const customers = useMemo(() => {
    const seen = new Map<string, string>();
    for (const i of integrations) {
      if (i.customer?.id && i.customer.name) {
        seen.set(i.customer.id, i.customer.name);
      }
    }
    return [...seen.entries()].map(([id, name]) => ({ id, name }));
  }, [integrations]);
  const totalCustomers = useMemo(
    () => new Set(integrations.map((i) => i.customer?.id ?? '')).size,
    [integrations]
  );

  if (totalCustomers <= 1) {
    return null;
  }
  return (
    <div className="relative select-none z-[500]" ref={ref}>
      <div
        data-tooltip-id="tooltip"
        data-tooltip-content={t('select_customer_tooltip', 'Select Customer')}
        onClick={openClose}
        className={clsx(
          'relative z-[20] cursor-pointer h-[42px] rounded-[8px] pl-[16px] pr-[12px] gap-[8px] border flex items-center',
          open ? 'border-[#612BD3]' : 'border-newColColor'
        )}
      >
        <div>
          <UserIcon />
        </div>
        <div>
          <DropdownArrowIcon rotated={open} />
        </div>
      </div>
      {open && (
        <div style={pos} className="flex flex-col fixed pt-[12px] bg-newBgColorInner menu-shadow min-w-[250px]">
          <div className="text-[14px] font-[600] px-[12px] mb-[5px]">{t('customers', 'Customers')}</div>
          {customers.map((p) => (
            <div
              onClick={() => {
                toaster.show(t('customer_socials_selected', 'Customer socials selected'), 'success');
                onChange(p.id);
                setOpen(false);
                onSelected?.();
              }}
              key={p.id}
              className="p-[12px] hover:bg-newBgColor text-[14px] font-[500] h-[32px] flex items-center cursor-pointer"
            >
              {p.name}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
