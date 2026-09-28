// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/missing-release.modal.tsx (AGPL-3.0).
// En lugar de elegir entre el contenido que devuelve la red, se pega el enlace de la publicación (F14).
import { FC, useCallback, useState } from 'react';
import { api } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useModals } from '@/components/ui/modals';
import { useToaster } from '@/components/ui/toaster';

export const MissingReleaseModal: FC<{ postId: string; onSuccess: () => void }> = ({ postId, onSuccess }) => {
  const t = useT();
  const modal = useModals();
  const toaster = useToaster();
  const [url, setUrl] = useState('');
  const [saving, setSaving] = useState(false);

  const handleSave = useCallback(async () => {
    setSaving(true);
    const { error } = await api.PUT('/posts/{id}/release', { params: { path: { id: postId } }, body: { url } });
    setSaving(false);
    if (error) {
      toaster.show(t(`release_error_${error.code}`, t('release_id_update_failed', 'Failed to connect post')), 'warning');
      return;
    }
    onSuccess();
    modal.closeCurrent();
  }, [modal, onSuccess, postId, t, toaster, url]);

  return (
    <div className="flex flex-col gap-[16px]">
      <div className="text-[14px] text-textColor/70">
        {t('paste_release_link', 'Paste the link to the publication that matches this post:')}
      </div>
      <Input
        name="url"
        label={t('publication_link', 'Publication link')}
        disableForm={true}
        value={url}
        placeholder="https://t.me/…"
        onChange={(e) => setUrl(e.target.value)}
        data-testid="release-url"
      />
      <div className="flex justify-end gap-[10px] pt-[8px] border-t border-tableBorder">
        <Button type="button" onClick={() => modal.closeCurrent()} className="bg-transparent border border-tableBorder text-textColor">
          {t('cancel', 'Cancel')}
        </Button>
        <Button type="button" onClick={handleSave} disabled={!url.trim() || saving} loading={saving}>
          {t('connect_post', 'Connect Post')}
        </Button>
      </div>
    </div>
  );
};
