// Adaptado de Postiz v2.24.0: apps/frontend/src/components/layout/top.menu.tsx y
// apps/frontend/src/components/new-layout/layout.component.tsx (AGPL-3.0).
import { ReactNode } from 'react';
import { useT } from '@/lib/i18n';
import { AppFrame } from '@/components/layout/app-frame';
import { LogoutMenuItem } from '@/components/layout/logout';
import { MenuItem } from '@/components/layout/menu-item';
import { ModeComponent } from '@/components/layout/mode';
import { OrganizationSelector } from '@/components/layout/organization-selector';

const CalendarIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="21" height="23" viewBox="0 0 21 23" fill="none">
    <path
      d="M19.5 9.5H1.5M14.5 1.5V5.5M6.5 1.5V5.5M6.3 21.5H14.7C16.3802 21.5 17.2202 21.5 17.862 21.173C18.4265 20.8854 18.8854 20.4265 19.173 19.862C19.5 19.2202 19.5 18.3802 19.5 16.7V8.3C19.5 6.61984 19.5 5.77976 19.173 5.13803C18.8854 4.57354 18.4265 4.1146 17.862 3.82698C17.2202 3.5 16.3802 3.5 14.7 3.5H6.3C4.61984 3.5 3.77976 3.5 3.13803 3.82698C2.57354 4.1146 2.1146 4.57354 1.82698 5.13803C1.5 5.77976 1.5 6.61984 1.5 8.3V16.7C1.5 18.3802 1.5 19.2202 1.82698 19.862C2.1146 20.4265 2.57354 20.8854 3.13803 21.173C3.77976 21.5 4.61984 21.5 6.3 21.5Z"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
);

export function HeaderActions() {
  return (
    <>
      <OrganizationSelector />
      <div className="hover:text-newTextColor">
        <ModeComponent />
      </div>
      <div className="w-[1px] h-[20px] bg-blockSeparator" />
    </>
  );
}

export function MemberFrame({ title, children }: { title: ReactNode; children: ReactNode }) {
  const t = useT();
  return (
    <AppFrame
      title={title}
      menu={<MenuItem path="/launches" label={t('calendar', 'Calendar')} icon={<CalendarIcon />} />}
      bottomMenu={<LogoutMenuItem />}
      headerActions={<HeaderActions />}
    >
      {children}
    </AppFrame>
  );
}
