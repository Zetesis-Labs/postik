// Adaptado de Postiz v2.24.0: apps/frontend/src/components/new-layout/layout.component.tsx (AGPL-3.0).
import { ReactNode } from 'react';
import { Logo } from '@/components/layout/logo';
import { LanguageComponent } from '@/components/layout/language';

export function AppFrame({
  title,
  menu,
  bottomMenu,
  headerActions,
  endActions,
  children,
}: {
  title: ReactNode;
  menu?: ReactNode;
  bottomMenu?: ReactNode;
  headerActions?: ReactNode;
  endActions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col min-h-screen min-w-screen text-newTextColor p-[12px]">
      <div className="flex-1 flex gap-[8px]">
        <div className="flex flex-col bg-newBgColorInner w-[80px] rounded-[12px]">
          <div id="left-menu" className="fixed h-full w-[64px] start-[17px] flex flex-1 top-0">
            <div className="flex flex-col h-full gap-[32px] flex-1 py-[12px]">
              <Logo />
              <div className="flex flex-1 flex-col minCustom:gap-[16px] blurMe">{menu}</div>
              <div className="flex flex-col minCustom:gap-[20px] custom:gap-[8px] blurMe">{bottomMenu}</div>
            </div>
          </div>
        </div>
        <div className="flex-1 bg-newBgLineColor rounded-[12px] overflow-hidden flex flex-col gap-[1px] blurMe">
          <div className="flex bg-newBgColorInner h-[80px] px-[20px] items-center">
            <div className="text-[24px] font-[600] flex flex-1">{title}</div>
            <div className="flex gap-[20px] items-center text-textItemBlur">
              {headerActions}
              <LanguageComponent />
              {endActions}
            </div>
          </div>
          <div className="flex flex-1 gap-[1px]">{children}</div>
        </div>
      </div>
    </div>
  );
}
