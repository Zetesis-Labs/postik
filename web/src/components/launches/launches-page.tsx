import { DndProvider } from 'react-dnd';
import { HTML5Backend } from 'react-dnd-html5-backend';
import { useT } from '@/lib/i18n';
import { MemberFrame } from '@/components/layout/member-frame';
import { ChannelsSidebar } from '@/components/launches/channels-sidebar';

export function LaunchesPage() {
  const t = useT();
  return (
    <MemberFrame title={t('calendar', 'Calendar')}>
      <DndProvider backend={HTML5Backend}>
        <ChannelsSidebar />
        <div className="bg-newBgColorInner flex-1 flex-col flex p-[20px] gap-[12px]" />
      </DndProvider>
    </MemberFrame>
  );
}
