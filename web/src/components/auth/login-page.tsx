// Adaptado de Postiz v2.24.0: apps/frontend/src/components/auth/login.tsx (AGPL-3.0).
// Solo OIDC y el acceso de administrador: sin registro, contraseña local ni recuperación.
import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { instanceQuery } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { AuthLayout } from '@/components/auth/auth-layout';
import { OauthProvider } from '@/components/auth/oauth-provider';

const knownErrors = ['oidc_denied', 'oidc_state', 'oidc_unavailable', 'email_required', 'invitation_required'];

export function LoginPage({ error }: { error?: string }) {
  const t = useT();
  const instance = useQuery(instanceQuery);
  const oidc = instance.data?.oidc;
  const message = error ? t(knownErrors.includes(error) ? error : 'something_went_wrong') : null;
  return (
    <AuthLayout>
      <div className="flex flex-col flex-1">
        <div>
          <h1 className="text-[40px] font-[500] -tracking-[0.8px] text-start">{t('sign_in', 'Sign In')}</h1>
        </div>
        {message && (
          <div role="alert" className="bg-red-500/10 border border-red-500/30 rounded-[10px] p-4 mt-[24px] text-red-400 text-sm">
            {message}
          </div>
        )}
        {oidc && (
          <>
            <div className="text-[14px] mt-[32px] mb-[12px]">{t('continue_with', 'Continue With')}</div>
            <div className="flex flex-col">
              <OauthProvider displayName={oidc.name} />
            </div>
          </>
        )}
        <p className="mt-[32px] text-sm">
          <Link to="/auth/admin" className="underline hover:font-bold cursor-pointer">
            {t('admin_access', 'Administrator access')}
          </Link>
        </p>
      </div>
    </AuthLayout>
  );
}
