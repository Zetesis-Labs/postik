// Adaptado de Postiz v2.24.0: apps/frontend/src/components/layout/logout.component.tsx (AGPL-3.0).
import { useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { api, meQuery } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { deleteDialog } from '@/components/ui/modals';
import { MenuItem } from '@/components/layout/menu-item';

const LogoutIcon = () => (
  <svg width="24" height="24" viewBox="0 0 25 24" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path
      d="M20.28 11.22C20.4205 11.3606 20.4993 11.5512 20.4993 11.75C20.4993 11.9488 20.4205 12.1394 20.28 12.28L11.28 21.28C11.1378 21.4125 10.9498 21.4846 10.7555 21.4812C10.5612 21.4777 10.3758 21.399 10.2384 21.2616C10.101 21.1242 10.0223 20.9388 10.0188 20.7445C10.0154 20.5502 10.0875 20.3622 10.22 20.22L17.9387 12.5H0.75C0.551088 12.5 0.360322 12.421 0.21967 12.2803C0.0790175 12.1397 0 11.9489 0 11.75C0 11.5511 0.0790175 11.3603 0.21967 11.2197C0.360322 11.079 0.551088 11 0.75 11H17.9387L10.22 3.28C10.0875 3.13783 10.0154 2.94978 10.0188 2.75548C10.0223 2.56118 10.101 2.37579 10.2384 2.23838C10.3758 2.10097 10.5612 2.02225 10.7555 2.01883C10.9498 2.0154 11.1378 2.08752 11.28 2.22L20.28 11.22ZM23.75 0C23.5511 0 23.3603 0.0790175 23.2197 0.21967C23.079 0.360322 23 0.551088 23 0.75V22.75C23 22.9489 23.079 23.1397 23.2197 23.2803C23.3603 23.421 23.5511 23.5 23.75 23.5C23.9489 23.5 24.1397 23.421 24.2803 23.2803C24.421 23.1397 24.5 22.9489 24.5 22.75V0.75C24.5 0.551088 24.421 0.360322 24.2803 0.21967C24.1397 0.0790175 23.9489 0 23.75 0Z"
      fill="currentColor"
    />
  </svg>
);

export function LogoutMenuItem() {
  const t = useT();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const logout = useCallback(async () => {
    const confirmed = await deleteDialog(
      t('are_you_sure_you_want_to_logout', 'Are you sure you want to logout?'),
      t('yes_logout', 'Yes logout')
    );
    if (!confirmed) {
      return;
    }
    await api.POST('/auth/logout');
    queryClient.setQueryData(meQuery.queryKey, null);
    await navigate({ to: '/auth/login' });
  }, [t, navigate, queryClient]);

  return <MenuItem label={t('logout', 'Log out')} icon={<LogoutIcon />} onClick={logout} />;
}
