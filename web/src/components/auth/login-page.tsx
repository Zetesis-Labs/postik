// Adaptado de Postiz v2.24.0: apps/frontend/src/components/auth/login.tsx (AGPL-3.0).
// Solo OIDC y el acceso de administrador: sin registro, contraseña local ni recuperación.
import { Link } from '@tanstack/react-router';
import { useT } from '@/lib/i18n';
import { AuthLayout } from '@/components/auth/auth-layout';

export function LoginPage() {
  const t = useT();
  return (
    <AuthLayout>
      <div className="flex flex-col flex-1">
        <div>
          <h1 className="text-[40px] font-[500] -tracking-[0.8px] text-start">{t('sign_in', 'Sign In')}</h1>
        </div>
        <p className="mt-[32px] text-sm">
          <Link to="/auth/admin" className="underline hover:font-bold cursor-pointer">
            {t('admin_access', 'Administrator access')}
          </Link>
        </p>
      </div>
    </AuthLayout>
  );
}
