// Adaptado de Postiz v2.24.0: apps/frontend/src/app/(app)/auth/layout.tsx (AGPL-3.0).
// Sin el texto promocional ni los testimonios de Postiz.
import { ReactNode } from 'react';
import { LanguageComponent } from '@/components/layout/language';
import { LogoText } from '@/components/layout/logo';

export function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="bg-[#0E0E0E] flex flex-1 p-[12px] gap-[12px] min-h-screen w-screen text-white">
      <div className="flex flex-col py-[40px] px-[20px] flex-1 lg:w-[600px] lg:flex-none rounded-[12px] text-white p-[12px] bg-[#1A1919]">
        <div className="w-full max-w-[440px] mx-auto justify-center gap-[20px] h-full flex flex-col text-white">
          <div className="flex items-center justify-between">
            <LogoText />
            <LanguageComponent />
          </div>
          <div className="flex">{children}</div>
        </div>
      </div>
      <div className="flex-1 hidden lg:flex" />
    </div>
  );
}
