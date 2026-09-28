// Lo que ve la persona al volver de la red (S06 §3 y §11). En Postiz lo hacen
// integrations/social/[provider]/page.tsx y layout/continue.provider.tsx.
import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { channelsQuery } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { useToaster } from '@/components/ui/toaster';
import { useContinuePage } from '@/components/launches/continue-page';

export function OAuthReturn() {
  const t = useT();
  const toaster = useToaster();
  const queryClient = useQueryClient();
  const continuePage = useContinuePage();

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const added = params.get('added');
    const pending = params.get('continue');
    const error = params.get('oauth_error');
    if (!added && !pending && !error) {
      return;
    }
    window.history.replaceState(window.history.state, '', window.location.pathname);
    void queryClient.invalidateQueries({ queryKey: channelsQuery.queryKey });
    if (error) {
      toaster.show(t(`oauth_error_${error}`, t('oauth_error_exchange_failed')), 'warning');
    } else if (pending) {
      continuePage(pending);
    } else {
      toaster.show(t('channel_added', 'Channel added'), 'success');
    }
  }, [continuePage, queryClient, t, toaster]);

  return null;
}
