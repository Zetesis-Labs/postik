import type { Media } from '@/lib/api';

export type UploadResult = { ok: true; media: Media } | { ok: false; code: string };

// uploadFile sends one file to the library with XMLHttpRequest, which, unlike
// fetch, reports upload progress.
export function uploadFile(file: File, onProgress: (fraction: number) => void): Promise<UploadResult> {
  return new Promise((resolve) => {
    const form = new FormData();
    form.append('file', file, file.name);
    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/v1/media');
    xhr.withCredentials = true;
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) {
        onProgress(event.loaded / event.total);
      }
    };
    xhr.onload = () => {
      try {
        const body = JSON.parse(xhr.responseText);
        resolve(xhr.status === 201 ? { ok: true, media: body } : { ok: false, code: body.code ?? 'unreadable' });
      } catch {
        resolve({ ok: false, code: 'unreadable' });
      }
    };
    xhr.onerror = () => resolve({ ok: false, code: 'unreadable' });
    xhr.send(form);
  });
}
