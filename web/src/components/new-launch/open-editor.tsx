import { dayjs } from '@/lib/dates';
import type { PostDetail } from '@/lib/api';
import { useModals } from '@/components/ui/modals';
import { Integrations } from '@/components/launches/calendar-context';
import { AddEditModal } from '@/components/new-launch/add-edit-modal';
import { ExistingDataContextProvider } from '@/components/new-launch/existing-data';
import { MediaItem } from '@/components/new-launch/store';

type Options = {
  integrations: Integrations[];
  mutate: () => void;
  date: dayjs.Dayjs;
  existing?: PostDetail;
  selectedChannels?: string[];
  onlyValues?: Array<{ content: string; media: Array<{ id: string; url: string; kind?: string; alt?: string }> }>;
};

// openPostEditor opens the editor the way Postiz does from the calendar, the
// channel menu and the «Create Post» button.
export function openPostEditor(modal: ReturnType<typeof useModals>, options: Options) {
  const existing = options.existing;
  const onlyValues = options.onlyValues?.map((v) => ({
    content: v.content,
    media: v.media.map<MediaItem>((m) => ({ id: m.id, path: m.url, kind: m.kind, alt: m.alt })),
  }));
  const editor = (
    <AddEditModal
      integrations={existing ? options.integrations.filter((i) => i.id === existing.channel.id) : options.integrations}
      allIntegrations={options.integrations}
      selectedChannels={options.selectedChannels}
      mutate={options.mutate}
      date={options.date}
      onlyValues={onlyValues}
    />
  );
  modal.openModal({
    id: 'add-edit-modal',
    removeLayout: true,
    withCloseButton: false,
    askClose: true,
    fullScreen: true,
    size: '80%',
    title: '',
    children: existing ? (
      <ExistingDataContextProvider value={{ integration: existing.channel.id, group: existing.groupId, post: existing }}>
        {editor}
      </ExistingDataContextProvider>
    ) : (
      editor
    ),
  });
}
