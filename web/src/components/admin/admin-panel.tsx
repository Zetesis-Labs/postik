import { useT } from '@/lib/i18n';
import { AppFrame } from '@/components/layout/app-frame';
import { LogoutMenuItem } from '@/components/layout/logout';
import { ModeComponent } from '@/components/layout/mode';

export function AdminPanel() {
  const t = useT();
  return (
    <AppFrame
      title={t('superadmin_panel', 'Superadmin panel')}
      bottomMenu={<LogoutMenuItem />}
      headerActions={
        <>
          <div className="hover:text-newTextColor">
            <ModeComponent />
          </div>
          <div className="w-[1px] h-[20px] bg-blockSeparator" />
        </>
      }
    >
      <div className="bg-newBgColorInner flex-1 p-[20px]">
        <p className="text-textItemBlur">{t('superadmin_panel_empty', 'The superadmin panel is still empty.')}</p>
      </div>
    </AppFrame>
  );
}
