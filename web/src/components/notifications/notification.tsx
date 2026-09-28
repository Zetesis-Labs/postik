// Adaptado de Postiz v2.24.0: apps/frontend/src/components/notifications/notification.component.tsx (AGPL-3.0).
// El texto se compone aquí, en el idioma de la pantalla, a partir de la plantilla y sus datos.
import { FC, useCallback, useState } from 'react';
import clsx from 'clsx';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, Notification } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { dayjs } from '@/lib/dates';
import { useClickOutside } from '@/components/launches/select-customer';

const notificationsKey = ['notifications'];

function escapeHtml(text: string) {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function replaceLinks(text: string) {
  const urlRegex = /(\bhttps?:\/\/[-A-Z0-9+&@#/%?=~_|!:,.;]*[-A-Z0-9+&@#/%=~_|])/gi;
  return text.replace(urlRegex, '<a class="cursor-pointer underline font-bold" target="_blank" rel="noreferrer" href="$1">$1</a>');
}

function useNotificationText() {
  const t = useT();
  return useCallback(
    (n: Notification) => {
      const provider = n.params.provider ? n.params.provider[0].toUpperCase() + n.params.provider.slice(1) : '';
      const reason = n.params.reason ? t(`post_error_${n.params.reason}`, n.params.reason) : '';
      return t(`notification_${n.template}`, n.template, { ...n.params, provider, reason, interpolation: { escapeValue: false } });
    },
    [t]
  );
}

export const ShowNotification: FC<{ notification: Notification }> = ({ notification }) => {
  const text = useNotificationText();
  const [newNotification] = useState(notification.unread);
  const createdAt = dayjs(notification.createdAt);
  const isWithin24h = dayjs().diff(createdAt, 'hour') < 24;
  const fullDate = createdAt.format('LLL');
  return (
    <div
      className={clsx(
        'text-textColor px-[16px] py-[10px] border-b border-tableBorder last:border-b-0 transition-colors',
        newNotification && 'font-bold bg-seventh animate-newMessages'
      )}
      data-testid="notification"
    >
      <div className="break-words" dangerouslySetInnerHTML={{ __html: replaceLinks(escapeHtml(text(notification))) }} />
      <div className="text-[11px] mt-[4px] opacity-60 font-normal" title={isWithin24h ? fullDate : undefined}>
        {isWithin24h ? createdAt.fromNow() : fullDate}
      </div>
    </div>
  );
};

export const NotificationOpenComponent: FC<{ items: Notification[] | undefined; loading: boolean }> = ({ items, loading }) => {
  const t = useT();
  return (
    <div
      id="notification-popup"
      className="opacity-0 animate-normalFadeDown mt-[10px] absolute w-[420px] min-h-[200px] top-[100%] end-0 bg-third text-textColor rounded-[16px] flex flex-col border border-tableBorder z-[600]"
    >
      <div className="p-[16px] border-b border-tableBorder font-bold">{t('notifications', 'Notifications')}</div>
      <div className="flex flex-col max-h-[400px] overflow-y-auto scrollbar scrollbar-thumb-fifth scrollbar-track-newBgColor">
        {loading && (
          <div className="flex-1 flex justify-center pt-12">
            <div className="animate-spin h-[36px] w-[36px] border-4 border-white border-t-transparent rounded-full" />
          </div>
        )}
        {!loading && !items?.length && (
          <div className="text-center p-[16px] text-textColor flex-1 flex justify-center items-center mt-[20px]">
            {t('no_notifications', 'No notifications')}
          </div>
        )}
        {!loading && items?.map((notification) => <ShowNotification notification={notification} key={notification.id} />)}
      </div>
    </div>
  );
};

export const NotificationComponent = () => {
  const t = useT();
  const queryClient = useQueryClient();
  const [show, setShow] = useState(false);
  const { data, isLoading } = useQuery({
    queryKey: notificationsKey,
    queryFn: async () => (await api.GET('/notifications')).data,
    refetchInterval: 60_000,
  });
  const [opened, setOpened] = useState<Notification[] | undefined>();

  const changeShow = useCallback(async () => {
    if (show) {
      setShow(false);
      return;
    }
    setOpened(data?.items);
    setShow(true);
    queryClient.setQueryData(notificationsKey, data ? { ...data, unread: 0 } : data);
    await api.POST('/notifications/read');
  }, [data, queryClient, show]);
  const close = useCallback(() => setShow(false), []);
  const ref = useClickOutside<HTMLDivElement>(close);

  return (
    <div className="relative cursor-pointer select-none" ref={ref}>
      <div onClick={() => void changeShow()} data-testid="notifications-bell" aria-label={t('notifications', 'Notifications')}>
        <svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" className="hover:text-newTextColor">
          <path
            d="M14 21H10M18 8C18 6.4087 17.3679 4.88258 16.2427 3.75736C15.1174 2.63214 13.5913 2 12 2C10.4087 2 8.8826 2.63214 7.75738 3.75736C6.63216 4.88258 6.00002 6.4087 6.00002 8C6.00002 11.0902 5.22049 13.206 4.34968 14.6054C3.61515 15.7859 3.24788 16.3761 3.26134 16.5408C3.27626 16.7231 3.31488 16.7926 3.46179 16.9016C3.59448 17 4.19261 17 5.38887 17H18.6112C19.8074 17 20.4056 17 20.5382 16.9016C20.6852 16.7926 20.7238 16.7231 20.7387 16.5408C20.7522 16.3761 20.3849 15.7859 19.6504 14.6054C18.7795 13.206 18 11.0902 18 8Z"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          {data && data.unread > 0 && (
            <circle cx="17.0625" cy="5" r="4" fill="#FF3EA2" stroke="#1A1919" strokeWidth="2" data-testid="notifications-unread" />
          )}
        </svg>
      </div>
      {show && <NotificationOpenComponent items={opened ?? data?.items} loading={isLoading} />}
    </div>
  );
};
