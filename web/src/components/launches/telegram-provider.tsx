// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/web3/providers/telegram.provider.tsx (AGPL-3.0).
// El código lo crea el servidor y la espera consulta el estado de la conexión.
import { FC, useCallback, useEffect, useRef, useState } from 'react';
import { api } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useToaster } from '@/components/ui/toaster';

const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

export const TelegramProvider: FC<{ onComplete: () => void }> = ({ onComplete }) => {
  const t = useT();
  const toaster = useToaster();
  const stop = useRef(false);
  const [connection, setConnection] = useState<{ code: string; bot: string } | null>(null);
  const [step, setStep] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void api.POST('/channels/telegram/connections').then(({ data }) => {
      if (cancelled) {
        return;
      }
      if (!data) {
        setProblem(t('something_went_wrong', 'Something went wrong. Please try again.'));
        return;
      }
      setConnection({ code: data.code, bot: data.botUsername });
    });
    return () => {
      cancelled = true;
      stop.current = true;
    };
  }, [t]);

  const loadAll = useCallback(async () => {
    if (!connection) {
      return;
    }
    stop.current = false;
    setStep(true);
    while (!stop.current) {
      const { data: status } = await api.GET('/channels/telegram/connections/{code}', {
        params: { path: { code: connection.code } },
      });
      if (stop.current) {
        return;
      }
      if (status?.status === 'connected') {
        onComplete();
        return;
      }
      if (status?.status === 'expired') {
        setProblem(t('telegram_connection_expired', 'The code expired. Close the window and try again.'));
        return;
      }
      await wait(2000);
    }
  }, [connection, onComplete, t]);

  const command = connection ? `/connect ${connection.code}` : '';
  const copyText = useCallback(() => {
    void navigator.clipboard?.writeText(command);
    toaster.show(t('copied_to_clipboard', 'Copied to clipboard'), 'success');
  }, [command, t, toaster]);

  return (
    <div className="justify-center items-center flex flex-col pt-[16px]">
      <div>
        {t('please_add', 'Please add')} <strong>@{connection?.bot}</strong>{' '}
        {t('to_your_telegram_group_channel_and_click_here', 'to your telegram group / channel and click here:')}
      </div>
      {!step ? (
        <div className="w-full mt-[16px]">
          <button
            type="button"
            onClick={loadAll}
            disabled={!connection}
            className={`w-full cursor-pointer bg-[#2EA6DD] h-[44px] rounded-[4px] flex justify-center items-center text-white gap-[4px]`}
          >
            <svg width="51" height="22" viewBox="0 0 72 63" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path
                d="M71.85 3.00001L60.612 60.378C60.612 60.378 60.129 63 56.877 63C55.149 63 54.258 62.178 54.258 62.178L29.916 41.979L18.006 35.976L2.721 31.911C2.721 31.911 0 31.125 0 28.875C0 27 2.799 26.106 2.799 26.106L66.747 0.70201C66.747 0.70201 68.7 -0.00299041 70.125 9.58803e-06C71.001 9.58803e-06 72 0.37501 72 1.50001C72 2.25001 71.85 3.00001 71.85 3.00001Z"
                fill="white"
              />
              <path
                d="M39.0005 49.5147L28.7225 59.6367C28.7225 59.6367 28.2755 59.9817 27.6785 59.9967C27.4715 60.0027 27.2495 59.9697 27.0215 59.8677L29.9135 41.9727L39.0005 49.5147Z"
                fill="#B0BEC5"
              />
              <path
                d="M59.691 12.5877C59.184 11.9277 58.248 11.8077 57.588 12.3087L18 35.9997C18 35.9997 24.318 53.6757 25.281 56.7357C26.247 59.7987 27.021 59.8707 27.021 59.8707L29.913 41.9757L59.409 14.6877C60.069 14.1867 60.192 13.2477 59.691 12.5877Z"
                fill="#CFD8DC"
              />
            </svg>
            <div>{t('connect_telegram', 'Connect Telegram')}</div>
          </button>
        </div>
      ) : (
        <div className="w-full text-center" onClick={copyText}>
          {t('please_add_the_following_command_in_your_chat', 'Please add the following command in your chat:')}
          <div className="mt-[16px] flex">
            <div className="flex-1">
              <Input label="" value={command} name="command" disableForm={true} readOnly data-testid="telegram-command" />
            </div>
            <Button>{t('copy', 'Copy')}</Button>
          </div>
        </div>
      )}
      {problem && <div className="text-red-400 text-[14px] mt-[12px]">{problem}</div>}
    </div>
  );
};
