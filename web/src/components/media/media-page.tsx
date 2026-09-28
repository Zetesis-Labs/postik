// Adaptado de Postiz v2.24.0: apps/frontend/src/components/new-layout/layout.media.component.tsx (AGPL-3.0).
import { useT } from '@/lib/i18n';
import { MemberFrame } from '@/components/layout/member-frame';
import { MediaBox } from '@/components/media/media-box';

export function MediaPage() {
  const t = useT();
  return (
    <MemberFrame title={t('media', 'Media')}>
      <div className="bg-newBgColorInner p-[20px] flex flex-1 flex-col gap-[15px] transition-all">
        <MediaBox setMedia={() => {}} standalone={true} />
      </div>
    </MemberFrame>
  );
}
