// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/new.post.tsx y la
// entrada «Create a new post» de apps/frontend/src/components/launches/menu/menu.tsx (AGPL-3.0).
// Sin sets: el editor se abre directamente en el siguiente hueco libre.
import { useCallback } from 'react';
import { api } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { newDayjs } from '@/lib/dates';
import { useModals } from '@/components/ui/modals';
import { useToaster } from '@/components/ui/toaster';
import { useCalendar } from '@/components/launches/calendar-context';
import { openPostEditor } from '@/components/new-launch/open-editor';

// useCreatePost opens the editor at the next free slot, of one channel when
// it is given and of every channel otherwise.
export function useCreatePost() {
  const modal = useModals();
  const toaster = useToaster();
  const t = useT();
  const { integrations, reloadCalendarView } = useCalendar();
  return useCallback(
    async (channelId?: string) => {
      const { data, error } = await api.GET('/posts/next-slot', { params: { query: channelId ? { channelId } : {} } });
      if (error) {
        toaster.show(t(error.code, error.message), 'warning');
        return;
      }
      openPostEditor(modal, {
        integrations,
        mutate: reloadCalendarView,
        date: newDayjs(data.date),
        selectedChannels: channelId ? [channelId] : undefined,
      });
    },
    [integrations, modal, reloadCalendarView, t, toaster]
  );
}

export const NewPost = () => {
  const t = useT();
  const createPost = useCreatePost();
  return (
    <button
      type="button"
      onClick={() => void createPost()}
      className="text-white flex-1 pt-[12px] pb-[14px] ps-[16px] pe-[20px] group-[.sidebar]:p-0 min-h-[44px] max-h-[44px] rounded-md bg-btnPrimary flex justify-center items-center gap-[5px] outline-none"
      data-testid="create-post"
    >
      <svg xmlns="http://www.w3.org/2000/svg" width="21" height="20" viewBox="0 0 21 20" fill="none" className="min-w-[21px] min-h-[20px]">
        <path d="M10.5001 4.16699V15.8337M4.66675 10.0003H16.3334" stroke="white" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
      <div className="flex-1 text-start text-[14px] group-[.sidebar]:hidden">{t('create_new_post', 'Create Post')}</div>
    </button>
  );
};
