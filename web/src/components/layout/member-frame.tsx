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

const MediaIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="20" height="21" viewBox="0 0 20 21" fill="none">
    <path
      d="M7.50008 3L6.66675 7.16667M13.3334 3L12.5001 7.16667M18.3334 7.16667H1.66675M5.66675 18H14.3334C15.7335 18 16.4336 18 16.9684 17.7275C17.4388 17.4878 17.8212 17.1054 18.0609 16.635C18.3334 16.1002 18.3334 15.4001 18.3334 14V7C18.3334 5.59987 18.3334 4.8998 18.0609 4.36502C17.8212 3.89462 17.4388 3.51217 16.9684 3.27248C16.4336 3 15.7335 3 14.3334 3H5.66675C4.26662 3 3.56655 3 3.03177 3.27248C2.56137 3.51217 2.17892 3.89462 1.93923 4.36502C1.66675 4.8998 1.66675 5.59987 1.66675 7V14C1.66675 15.4001 1.66675 16.1002 1.93923 16.635C2.17892 17.1054 2.56137 17.4878 3.03177 17.7275C3.56655 18 4.26662 18 5.66675 18Z"
      stroke="currentColor"
      strokeWidth="1.5"
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
      menu={
        <>
          <MenuItem path="/launches" label={t('calendar', 'Calendar')} icon={<CalendarIcon />} />
          <MenuItem path="/media" label={t('media', 'Media')} icon={<MediaIcon />} />
        </>
      }
      bottomMenu={<LogoutMenuItem />}
      headerActions={<HeaderActions />}
    >
      {children}
    </AppFrame>
  );
}
