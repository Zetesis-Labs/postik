// Adaptado de Postiz v2.24.0: apps/frontend/src/components/layout/settings.component.tsx,
// apps/frontend/src/components/settings/global.settings.tsx y
// apps/frontend/src/components/settings/email-notifications.component.tsx (AGPL-3.0).
// Por ahora solo la pestaña «Ajustes generales» con los correos; el equipo llega en S09.
import { useCallback, useEffect, useRef, useState } from 'react';
import clsx from 'clsx';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, meQuery } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { MemberFrame } from '@/components/layout/member-frame';
import { Slider } from '@/components/ui/slider';
import { useToaster } from '@/components/ui/toaster';
import { SVGLine } from '@/components/launches/channels-sidebar';

interface EmailNotifications {
  emailSuccess: boolean;
  emailFailure: boolean;
}

const EmailNotificationsComponent = () => {
  const t = useT();
  const toaster = useToaster();
  const queryClient = useQueryClient();
  const { data, isLoading } = useQuery(meQuery);
  const [local, setLocal] = useState<EmailNotifications>({ emailSuccess: true, emailFailure: true });
  const settingsRef = useRef(local);
  settingsRef.current = local;

  useEffect(() => {
    if (data?.user) {
      setLocal({ emailSuccess: data.user.emailSuccess, emailFailure: data.user.emailFailure });
    }
  }, [data]);

  const updateSetting = useCallback(
    async (key: keyof EmailNotifications, value: boolean) => {
      const next = { ...settingsRef.current, [key]: value };
      setLocal(next);
      const { error } = await api.PUT('/me/preferences', { body: next });
      if (error) {
        toaster.show(t(error.code, error.message), 'warning');
        return;
      }
      void queryClient.invalidateQueries({ queryKey: meQuery.queryKey });
      toaster.show(t('settings_updated', 'Settings updated'), 'success');
    },
    [queryClient, t, toaster]
  );

  if (isLoading) {
    return (
      <div className="my-[16px] mt-[16px] bg-sixth border-fifth border rounded-[4px] p-[24px]">
        <div className="animate-pulse">{t('loading', 'Loading...')}</div>
      </div>
    );
  }

  return (
    <div className="my-[16px] mt-[16px] bg-sixth border-fifth border rounded-[4px] p-[24px] flex flex-col gap-[24px]">
      <div className="mt-[4px]">{t('email_notifications', 'Email Notifications')}</div>
      <div className="flex items-center justify-between">
        <div className="flex flex-col">
          <div className="text-[14px]">{t('success_emails', 'Success Emails')}</div>
          <div className="text-[12px] text-customColor18">
            {t('success_emails_description', 'Receive email notifications when posts are published successfully')}
          </div>
        </div>
        <Slider
          label={t('success_emails', 'Success Emails')}
          value={local.emailSuccess ? 'on' : 'off'}
          onChange={(value) => void updateSetting('emailSuccess', value === 'on')}
          fill={true}
        />
      </div>
      <div className="flex items-center justify-between">
        <div className="flex flex-col">
          <div className="text-[14px]">{t('failure_emails', 'Failure Emails')}</div>
          <div className="text-[12px] text-customColor18">
            {t('failure_emails_description', 'Receive email notifications when posts fail to publish')}
          </div>
        </div>
        <Slider
          label={t('failure_emails', 'Failure Emails')}
          value={local.emailFailure ? 'on' : 'off'}
          onChange={(value) => void updateSetting('emailFailure', value === 'on')}
          fill={true}
        />
      </div>
    </div>
  );
};

const GlobalSettings = () => {
  const t = useT();
  return (
    <div className="flex flex-col">
      <h3 className="text-[20px]">{t('global_settings', 'Global Settings')}</h3>
      <EmailNotificationsComponent />
    </div>
  );
};

export function SettingsPage() {
  const t = useT();
  const [tab, setTab] = useState('global_settings');
  const list = [{ tab: 'global_settings', label: t('global_settings', 'Global Settings') }];
  return (
    <MemberFrame title={t('settings', 'Settings')}>
      <div className="bg-newBgColorInner p-[20px] flex flex-col transition-all w-[260px]">
        <div className="flex flex-1 flex-col gap-[15px]">
          {list.map(({ tab: tabKey, label }) => (
            <div
              key={tabKey}
              className={clsx('cursor-pointer flex items-center gap-[12px] group/profile hover:bg-boxHover rounded-e-[8px]', tabKey === tab && 'bg-boxHover')}
              onClick={() => setTab(tabKey)}
            >
              <div className={clsx('h-full w-[4px] rounded-s-[3px] opacity-0 group-hover/profile:opacity-100 transition-opacity', tabKey === tab && 'opacity-100')}>
                <SVGLine />
              </div>
              {label}
            </div>
          ))}
        </div>
      </div>
      <div className="bg-newBgColorInner flex-1 flex-col flex p-[20px] gap-[12px]">
        <div className="w-full mx-auto gap-[24px] flex flex-col relative rounded-[4px]">
          {tab === 'global_settings' && (
            <div>
              <GlobalSettings />
            </div>
          )}
        </div>
      </div>
    </MemberFrame>
  );
}
