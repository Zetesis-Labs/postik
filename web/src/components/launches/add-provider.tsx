// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/add.provider.component.tsx (AGPL-3.0).
// Sin el enlace de invitación (API pública, fuera de la v1) ni las vías web3, extensión o URL propia.
import { FC, useCallback } from 'react';
import clsx from 'clsx';
import { useQueryClient } from '@tanstack/react-query';
import { channelsQuery, Provider, providersQuery } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { useModals } from '@/components/ui/modals';
import { useToaster } from '@/components/ui/toaster';
import { TelegramProvider } from '@/components/launches/telegram-provider';

export const useAddProvider = () => {
  const modal = useModals();
  const queryClient = useQueryClient();
  const t = useT();
  return useCallback(async () => {
    const social = await queryClient.fetchQuery(providersQuery);
    modal.openModal({
      title: t('add_channel', 'Add Channel'),
      withCloseButton: true,
      children: <AddProviderComponent social={social} />,
    });
  }, [modal, queryClient, t]);
};

export const AddProviderButton: FC = () => {
  const add = useAddProvider();
  const t = useT();
  return (
    <div className="flex group-[.sidebar]:block gap-[8px]">
      <button
        type="button"
        className="flex-1 group-[.sidebar]:w-[100%] group-[.sidebar]:flex-none text-btnText bg-btnSimple h-[44px] pt-[12px] pb-[14px] ps-[16px] pe-[20px] justify-center items-center flex rounded-[8px] gap-[8px]"
        onClick={add}
      >
        <div>
          <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 20 20" fill="none">
            <path
              d="M1.66675 10.0417C3.35907 10.2299 4.93698 10.9884 6.14101 12.1924C7.34504 13.3964 8.10353 14.9743 8.29175 16.6667M1.66675 13.4167C2.46749 13.58 3.20253 13.9751 3.7804 14.553C4.35827 15.1309 4.75344 15.8659 4.91675 16.6667M1.66675 16.6667H1.67508M11.6667 17.5H14.3334C15.7335 17.5 16.4336 17.5 16.9684 17.2275C17.4388 16.9878 17.8212 16.6054 18.0609 16.135C18.3334 15.6002 18.3334 14.9001 18.3334 13.5V6.5C18.3334 5.09987 18.3334 4.3998 18.0609 3.86502C17.8212 3.39462 17.4388 3.01217 16.9684 2.77248C16.4336 2.5 15.7335 2.5 14.3334 2.5H5.66675C4.26662 2.5 3.56655 2.5 3.03177 2.77248C2.56137 3.01217 2.17892 3.39462 1.93923 3.86502C1.66675 4.3998 1.66675 5.09987 1.66675 6.5V6.66667"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </div>
        <div className="text-start text-[14px] group-[.sidebar]:hidden">{t('add_channel', 'Add Channel')}</div>
      </button>
    </div>
  );
};

export const AddProviderComponent: FC<{ social: Provider[] }> = ({ social }) => {
  const modal = useModals();
  const toaster = useToaster();
  const queryClient = useQueryClient();
  const t = useT();

  const openProvider = useCallback(
    (identifier: string) => () => {
      if (identifier !== 'telegram') {
        return;
      }
      modal.openModal({
        title: t('add_telegram', 'Add Telegram'),
        withCloseButton: true,
        children: (
          <TelegramProvider
            onComplete={() => {
              modal.closeAll();
              toaster.show(t('channel_added', 'Channel added'), 'success');
              void queryClient.invalidateQueries({ queryKey: channelsQuery.queryKey });
            }}
          />
        ),
      });
    },
    [modal, queryClient, t, toaster]
  );

  return (
    <div className="w-full flex flex-col gap-[20px] rounded-[4px] relative">
      <div className="flex flex-col">
        <div className={clsx('grid grid-cols-5 gap-[10px] justify-items-center justify-center')}>
          {social.map((item) => (
            <button
              type="button"
              key={item.identifier}
              onClick={openProvider(item.identifier)}
              className={clsx(
                'flex-col p-[10px] h-[100px] justify-center',
                'w-full text-[14px] rounded-[8px] bg-newTableHeader text-textColor relative items-center flex gap-[10px] cursor-pointer'
              )}
            >
              <div>
                <img className={clsx('w-[32px] h-[32px] rounded-full')} src={`/icons/platforms/${item.identifier}.png`} alt="" />
              </div>
              <div className={clsx('whitespace-pre-wrap', 'text-center')}>{item.name}</div>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
};
