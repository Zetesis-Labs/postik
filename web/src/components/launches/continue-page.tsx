// Adaptado de Postiz v2.24.0: apps/frontend/src/components/new-launch/providers/continue-provider/with-continue-provider.tsx,
// linkedin/linkedin.continue.tsx y apps/frontend/src/components/layout/continue.provider.tsx (AGPL-3.0).
// Las páginas ya conectadas no se esconden: elegir una actualiza su canal (S06 §3, paso 5).
import { FC, useCallback, useState } from 'react';
import clsx from 'clsx';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, channelsQuery } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { Button } from '@/components/ui/button';
import { useModals } from '@/components/ui/modals';
import { useToaster } from '@/components/ui/toaster';

const emptyStateMessages = [
  { key: 'we_couldn_t_find_any_business_connected_to_your_linkedin_page', text: "We couldn't find any business connected to your LinkedIn Page." },
  { key: 'please_close_this_dialog_create_a_new_page_and_add_a_new_channel_again', text: 'Please close this dialog, create a new page, and add a new channel again.' },
];

export const ContinuePage: FC<{ channelId: string; onDone: () => void }> = ({ channelId, onDone }) => {
  const t = useT();
  const toaster = useToaster();
  const queryClient = useQueryClient();
  const [selection, setSelection] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const { data, isLoading, isError } = useQuery({
    queryKey: ['channel-pages', channelId],
    queryFn: async () => {
      const { data, response } = await api.GET('/channels/{id}/pages', { params: { path: { id: channelId } } });
      if (!data) {
        throw new Error(`GET /channels/${channelId}/pages answered ${response.status}`);
      }
      return data;
    },
    retry: false,
  });

  const save = useCallback(async () => {
    if (!selection) {
      return;
    }
    setSaving(true);
    const { response } = await api.PUT('/channels/{id}/page', { params: { path: { id: channelId } }, body: { pageId: selection } });
    setSaving(false);
    if (!response.ok) {
      toaster.show(t('page_not_saved', 'The page could not be saved'), 'warning');
      return;
    }
    await queryClient.invalidateQueries({ queryKey: channelsQuery.queryKey });
    toaster.show(t('channel_added', 'Channel added'), 'success');
    onDone();
  }, [channelId, onDone, queryClient, selection, t, toaster]);

  if (isLoading) {
    return <div className="h-[300px]" />;
  }
  if (isError || !data?.length) {
    return (
      <div className="text-center flex flex-col justify-center items-center text-[18px] leading-[26px] h-[300px]">
        {emptyStateMessages.map((msg, index) => (
          <span key={msg.key}>
            {t(msg.key, msg.text)}
            {index < emptyStateMessages.length - 1 && (
              <>
                <br />
                <br />
              </>
            )}
          </span>
        ))}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[20px]" data-testid="continue-page">
      <div>{t('select_linkedin_page', 'Select Linkedin Page:')}</div>
      <div className="grid grid-cols-3 justify-items-center select-none cursor-pointer gap-[10px]">
        {data.map((page) => (
          <div
            key={page.id}
            role="option"
            aria-selected={selection === page.id}
            className={clsx(
              'flex flex-col w-full text-center gap-[10px] border border-input p-[10px] hover:bg-seventh rounded-[8px]',
              selection === page.id && 'bg-seventh border-primary'
            )}
            onClick={() => setSelection(page.id)}
          >
            <div>
              <img className="w-full" src={page.picture || '/no-picture.jpg'} alt="profile" />
            </div>
            <div>{page.name}</div>
          </div>
        ))}
      </div>
      <div>
        <Button disabled={!selection || saving} loading={saving} onClick={() => void save()}>
          {t('save', 'Save')}
        </Button>
      </div>
    </div>
  );
};

// useContinuePage opens the grid of pages of a channel in between steps.
export const useContinuePage = () => {
  const modals = useModals();
  const t = useT();
  return useCallback(
    (channelId: string) => {
      modals.openModal({
        id: `continue-${channelId}`,
        title: t('configure_channel', 'Configure Channel'),
        withCloseButton: true,
        children: (close) => <ContinuePage channelId={channelId} onDone={close} />,
      });
    },
    [modals, t]
  );
};
