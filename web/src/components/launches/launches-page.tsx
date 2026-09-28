import { useMemo } from 'react';
import { DndProvider } from 'react-dnd';
import { HTML5Backend } from 'react-dnd-html5-backend';
import { useQuery } from '@tanstack/react-query';
import { channelsQuery } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { MemberFrame } from '@/components/layout/member-frame';
import { ChannelsSidebar } from '@/components/launches/channels-sidebar';
import { CalendarWeekProvider, toIntegration } from '@/components/launches/calendar-context';
import { Filters } from '@/components/launches/filters';
import { Calendar } from '@/components/launches/calendar';

export function LaunchesPage() {
  const t = useT();
  const { data: channels = [] } = useQuery(channelsQuery);
  const integrations = useMemo(() => channels.map(toIntegration), [channels]);
  return (
    <MemberFrame title={t('calendar', 'Calendar')}>
      <DndProvider backend={HTML5Backend}>
        <CalendarWeekProvider integrations={integrations}>
          <ChannelsSidebar />
          <div className="bg-newBgColorInner flex-1 flex-col flex p-[20px] gap-[12px]">
            <Filters />
            <div className="flex-1 flex">
              <Calendar />
            </div>
          </div>
        </CalendarWeekProvider>
      </DndProvider>
    </MemberFrame>
  );
}
