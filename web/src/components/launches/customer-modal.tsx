// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/customer.modal.tsx (AGPL-3.0).
import { FC, useCallback, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api, Channel, customersQuery } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { Button } from '@/components/ui/button';
import { useModals } from '@/components/ui/modals';

export const CustomerModal: FC<{ integration: Channel; onClose: () => void }> = ({ integration, onClose }) => {
  const t = useT();
  const modal = useModals();
  const [customer, setCustomer] = useState(integration.customer?.name || undefined);
  const { data } = useQuery(customersQuery);

  const saveCustomer = useCallback(
    async (removeCustomer?: boolean) => {
      if (!customer && !removeCustomer) {
        return;
      }
      await api.PUT('/channels/{id}/customer', {
        params: { path: { id: integration.id } },
        body: { name: removeCustomer ? '' : customer },
      });
      modal.closeAll();
      onClose();
    },
    [customer, integration.id, modal, onClose]
  );

  return (
    <div className="relative w-full">
      <div className="mb-[80px] flex flex-col gap-[6px]">
        <label htmlFor="customer-name" className="text-[14px] text-white">
          {t('select_customer_label', 'Select Customer')}
        </label>
        <input
          id="customer-name"
          list="customer-name-options"
          value={customer ?? ''}
          onChange={(e) => setCustomer(e.target.value)}
          placeholder={t('start_typing', 'Start typing...')}
          autoComplete="off"
          className="bg-newBgColorInner h-[42px] border-newTableBorder border rounded-[8px] text-textColor placeholder-textColor px-[16px] text-[14px] outline-none"
        />
        <datalist id="customer-name-options">
          {(data ?? []).map((c) => (
            <option key={c.id} value={c.name} />
          ))}
        </datalist>
      </div>

      <div className="my-[16px] flex gap-[10px]">
        <Button onClick={() => saveCustomer()}>{t('save', 'Save')}</Button>
        {!!integration.customer?.name && (
          <Button className="bg-red-700" onClick={() => saveCustomer(true)}>
            {t('remove_from_customer', 'Remove from customer')}
          </Button>
        )}
      </div>
    </div>
  );
};
