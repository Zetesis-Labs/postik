import { useT } from '@/lib/i18n';
import { MemberFrame } from '@/components/layout/member-frame';

export function LaunchesPage() {
  const t = useT();
  return (
    <MemberFrame title={t('calendar', 'Calendar')}>
      <div className="bg-newBgColorInner flex-1" />
    </MemberFrame>
  );
}
