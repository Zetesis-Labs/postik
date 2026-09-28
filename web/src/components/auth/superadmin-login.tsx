// Adaptado de Postiz v2.24.0: apps/frontend/src/components/auth/login.tsx (AGPL-3.0).
import { useState } from 'react';
import { FormProvider, SubmitHandler, useForm } from 'react-hook-form';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from '@tanstack/react-router';
import { api, instanceQuery, meQuery } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { AuthLayout } from '@/components/auth/auth-layout';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

type Inputs = {
  username: string;
  password: string;
  code: string;
};

export function SuperadminLoginPage() {
  const t = useT();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const instance = useQuery(instanceQuery);
  const [loading, setLoading] = useState(false);
  const form = useForm<Inputs>({ defaultValues: { username: '', password: '', code: '' } });

  const onSubmit: SubmitHandler<Inputs> = async (inputs) => {
    setLoading(true);
    try {
      const { data, response } = await api.POST('/auth/superadmin', {
        body: {
          username: inputs.username,
          password: inputs.password,
          ...(inputs.code ? { code: inputs.code } : {}),
        },
      });
      if (data) {
        queryClient.setQueryData(meQuery.queryKey, data);
        await navigate({ to: '/admin' });
        return;
      }
      form.setError('username', {
        message:
          response.status === 401
            ? t('invalid_credentials', 'The details are not correct')
            : t('something_went_wrong', 'Something went wrong. Please try again.'),
      });
    } catch {
      form.setError('username', { message: t('something_went_wrong', 'Something went wrong. Please try again.') });
    } finally {
      setLoading(false);
    }
  };

  return (
    <AuthLayout>
      <FormProvider {...form}>
        <form className="flex-1 flex" onSubmit={form.handleSubmit(onSubmit)}>
          <div className="flex flex-col flex-1">
            <div>
              <h1 className="text-[40px] font-[500] -tracking-[0.8px] text-start">
                {t('admin_access_title', 'Administrator access')}
              </h1>
            </div>
            <div className="flex flex-col gap-[12px] mt-[32px]">
              <div className="text-textColor">
                <Input label="Username" translationKey="label_username" name="username" autoComplete="username" />
                <Input
                  label="Password"
                  translationKey="label_password"
                  name="password"
                  type="password"
                  autoComplete="current-password"
                />
                {instance.data?.superadminTotp && (
                  <Input
                    label="Code"
                    translationKey="label_code"
                    name="code"
                    autoComplete="one-time-code"
                    placeholder={t('code_placeholder', 'TOTP or recovery code')}
                  />
                )}
              </div>
              <div className="text-center mt-6">
                <div className="w-full flex">
                  <Button type="submit" className="flex-1 rounded-[10px] !h-[52px]" loading={loading}>
                    {t('enter', 'Sign in')}
                  </Button>
                </div>
                <p className="mt-4 text-sm">
                  <Link to="/auth/login" className="underline hover:font-bold cursor-pointer">
                    {t('back_to_sign_in', 'Back to sign in')}
                  </Link>
                </p>
              </div>
            </div>
          </div>
        </form>
      </FormProvider>
    </AuthLayout>
  );
}
