// Adaptado de Postiz v2.24.0: apps/frontend/src/components/auth/providers/oauth.provider.tsx (AGPL-3.0).
import { useT } from '@/lib/i18n';

export const OauthProvider = ({ displayName }: { displayName: string }) => {
  const t = useT();
  const gotoLogin = () => {
    window.location.href = '/api/v1/auth/oidc/login';
  };
  return (
    <button
      type="button"
      onClick={gotoLogin}
      className={`cursor-pointer flex-1 bg-white h-[44px] rounded-[4px] flex justify-center items-center text-customColor16 gap-[4px]`}
    >
      <div>
        <img src="/icons/generic-oauth.svg" alt="" width={40} height={40} className="-mt-[7px]" />
      </div>
      <div>
        {t('sign_in_with', 'Sign in with')}&nbsp;
        {displayName}
      </div>
    </button>
  );
};
