// Adaptado de Postiz v2.24.0: apps/frontend/src/components/media/media.component.tsx,
// apps/frontend/src/components/layout/drop.files.tsx y
// libraries/react-shared-libraries/src/helpers/video.frame.tsx (AGPL-3.0).
// Uppy se sustituye por un <input type="file"> nativo con subida por XMLHttpRequest.
import { ChangeEvent, DragEvent, FC, ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import clsx from 'clsx';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, Media, mediaQuery } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { uploadFile } from '@/lib/upload';
import { ChevronLeftIcon, ChevronRightIcon, DeleteCircleIcon, NoMediaIcon, PlusIcon } from '@/components/ui/icons';
import { deleteDialog, useModals } from '@/components/ui/modals';
import { useToaster } from '@/components/ui/toaster';
import { storedMode } from '@/components/layout/mode';

const MAX_UPLOAD_SIZE = 1024 * 1024 * 1024;

export const VideoFrame: FC<{ url: string; autoplay?: boolean }> = ({ url, autoplay }) => (
  <video className="w-full h-full object-cover rounded-[4px]" src={url + '#t=0.1'} preload="metadata" autoPlay={!!autoplay} />
);

const DropFiles: FC<{ children: ReactNode; className?: string; onDrop: (files: File[]) => void; disabled?: boolean }> = (
  props
) => {
  const t = useT();
  const toaster = useToaster();
  const [isDragActive, setDragActive] = useState(false);
  const onDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setDragActive(false);
    if (props.disabled) {
      toaster.show('Upload current in progress, please wait and then try again.', 'warning');
      return;
    }
    props.onDrop(Array.from(e.dataTransfer.files));
  };
  return (
    <div
      className={clsx('relative', props.className)}
      onDragOver={(e) => {
        e.preventDefault();
        setDragActive(true);
      }}
      onDragLeave={() => setDragActive(false)}
      onDrop={onDrop}
    >
      {isDragActive && (
        <div className="absolute start-0 top-0 w-full h-full bg-black/90 flex items-center justify-center z-[200] animate-normalFadeIn">
          {t('drag_n_drop_some_files_here', 'Drag n drop some files here')}
        </div>
      )}
      {props.children}
    </div>
  );
};

export const Pagination: FC<{ current: number; totalPages: number; setPage: (num: number) => void }> = ({
  current,
  totalPages,
  setPage,
}) => {
  const t = useT();
  const paginationItems = useMemo(() => {
    const c = current + 1;
    const m = totalPages;
    if (m <= 10) {
      return Array.from({ length: m }, (_, i) => i + 1);
    }
    const delta = 3;
    const left = c - delta;
    const right = c + delta + 1;
    const range: number[] = [];
    const rangeWithDots: (number | '...')[] = [];
    let l: number | undefined;
    for (let i = 1; i <= m; i++) {
      if (i === 1 || i === m || (i >= left && i < right)) {
        range.push(i);
      }
    }
    for (const i of range) {
      if (l !== undefined) {
        if (i - l === 2) {
          rangeWithDots.push(l + 1);
        } else if (i - l !== 1) {
          rangeWithDots.push('...');
        }
      }
      rangeWithDots.push(i);
      l = i;
    }
    while (rangeWithDots.length > 10) {
      const currentIndex = rangeWithDots.findIndex((item) => item === c);
      if (currentIndex !== -1 && currentIndex > rangeWithDots.length / 2) {
        rangeWithDots.splice(2, 1);
      } else {
        rangeWithDots.splice(-3, 1);
      }
    }
    return rangeWithDots;
  }, [current, totalPages]);

  return (
    <ul className="flex flex-row items-center gap-1 justify-center mt-[15px]">
      <li className={clsx(current === 0 && 'opacity-20 pointer-events-none')}>
        <div
          className="cursor-pointer inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0 h-10 px-4 py-2 gap-1 ps-2.5 text-gray-400 hover:text-white border-[#1F1F1F] hover:bg-forth"
          aria-label="Go to previous page"
          onClick={() => setPage(current - 1)}
        >
          <ChevronLeftIcon className="lucide lucide-chevron-left h-4 w-4" />
          <span>{t('previous', 'Previous')}</span>
        </div>
      </li>
      {paginationItems.map((item, index) => (
        <li key={index}>
          {item === '...' ? (
            <span className="inline-flex items-center justify-center h-10 w-10 text-textColor select-none">...</span>
          ) : (
            <div
              aria-current="page"
              onClick={() => setPage(item - 1)}
              className={clsx(
                'cursor-pointer inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0 border hover:bg-forth h-10 w-10 hover:text-white border-newBorder',
                current === item - 1 ? 'bg-forth !text-white' : 'text-textColor hover:text-white'
              )}
            >
              {item}
            </div>
          )}
        </li>
      ))}
      <li className={clsx(current + 1 === totalPages && 'opacity-20 pointer-events-none')}>
        <a
          className="text-textColor hover:text-white group cursor-pointer inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0 h-10 px-4 py-2 gap-1 pe-2.5 text-gray-400 border-[#1F1F1F] hover:bg-forth"
          aria-label="Go to next page"
          onClick={() => setPage(current + 1)}
        >
          <span>{t('next', 'Next')}</span>
          <ChevronRightIcon className="lucide lucide-chevron-right h-4 w-4" />
        </a>
      </li>
    </ul>
  );
};

type Upload = { name: string; progress: number };

function useDebounced<T>(value: T, ms: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), ms);
    return () => clearTimeout(timer);
  }, [value, ms]);
  return debounced;
}

export const MediaBox: FC<{
  setMedia: (media: Media[]) => void;
  standalone?: boolean;
  type?: 'image' | 'video';
}> = ({ type, standalone, setMedia }) => {
  const t = useT();
  const modals = useModals();
  const toaster = useToaster();
  const queryClient = useQueryClient();
  const [page, setPage] = useState(0);
  const [search, setSearch] = useState('');
  const debouncedSearch = useDebounced(search.trim(), 300);
  const [selected, setSelected] = useState<Media[]>([]);
  const [uploads, setUploads] = useState<Upload[]>([]);
  const uploaderRef = useRef<HTMLInputElement>(null);
  const loading = uploads.length > 0;

  useEffect(() => {
    setPage(0);
  }, [debouncedSearch]);

  const { data, isLoading } = useQuery(mediaQuery(page + 1, debouncedSearch));
  const refresh = useCallback(() => queryClient.invalidateQueries({ queryKey: ['media'] }), [queryClient]);

  const uploadAll = useCallback(
    async (files: File[]) => {
      const totalSize = files.reduce((acc, file) => acc + file.size, 0);
      if (totalSize > MAX_UPLOAD_SIZE) {
        toaster.show(t('upload_size_limit_exceeded', 'Upload size limit exceeded. Maximum 1 GB per upload session.'), 'warning');
        return;
      }
      setUploads(files.map((f) => ({ name: f.name, progress: 0 })));
      const uploaded: Media[] = [];
      for (const [index, file] of files.entries()) {
        const result = await uploadFile(file, (progress) =>
          setUploads((prev) => prev.map((u, i) => (i === index ? { ...u, progress } : u)))
        );
        if (result.ok) {
          uploaded.push(result.media);
        } else {
          toaster.show(`${file.name}: ${t(result.code)}`, 'warning');
        }
      }
      setUploads([]);
      await refresh();
      if (!standalone && uploaded.length) {
        setSelected((prev) => [...prev, ...uploaded]);
      }
    },
    [refresh, standalone, t, toaster]
  );

  const addToUpload = useCallback(
    (e: ChangeEvent<HTMLInputElement>) => {
      const files = Array.from(e.target.files || []);
      e.target.value = '';
      void uploadAll(files);
    },
    [uploadAll]
  );

  const addRemoveSelected = useCallback(
    (media: Media) => () => {
      if (standalone) {
        return;
      }
      setSelected((prev) => (prev.some((p) => p.id === media.id) ? prev.filter((f) => f.id !== media.id) : [...prev, media]));
    },
    [standalone]
  );

  const addMedia = useCallback(() => {
    if (standalone) {
      return;
    }
    setMedia(selected);
    modals.closeCurrent();
  }, [modals, selected, setMedia, standalone]);

  const maximize = useCallback(
    (media: Media) => (e: { stopPropagation: () => void }) => {
      e.stopPropagation();
      modals.openModal({
        title: '',
        top: 10,
        children: (
          <div className="w-full h-full p-[50px]">
            {media.kind === 'video' ? (
              <VideoFrame autoplay={true} url={media.url} />
            ) : (
              <img width="100%" height="100%" className="w-full h-full max-h-[100%] max-w-[100%] object-cover" src={media.url} alt="media" />
            )}
          </div>
        ),
      });
    },
    [modals]
  );

  const deleteImage = useCallback(
    (media: Media) => async (e: { stopPropagation: () => void }) => {
      e.stopPropagation();
      if (!(await deleteDialog(t('are_you_sure_you_want_to_delete_the_image', 'Are you sure you want to delete the image?')))) {
        return;
      }
      await api.DELETE('/media/{id}', { params: { path: { id: media.id } } });
      await refresh();
    },
    [refresh, t]
  );

  const results = (data?.items ?? []).filter((f) => (type ? f.kind === type : true));
  const empty = !isLoading && !data?.items?.length;

  const btn = (
    <button
      type="button"
      disabled={loading}
      onClick={() => uploaderRef.current?.click()}
      className="relative cursor-pointer bg-btnSimple changeColor flex gap-[8px] h-[44px] px-[18px] justify-center items-center rounded-[8px]"
    >
      {loading ? (
        <div className="absolute left-[50%] top-[50%] -translate-y-[50%] -translate-x-[50%]">
          <div className="animate-spin h-[20px] w-[20px] border-4 border-white border-t-transparent rounded-full" />
        </div>
      ) : (
        <PlusIcon size={14} />
      )}
      <div className={loading ? 'invisible' : undefined}>{t('upload', 'Upload')}</div>
    </button>
  );

  return (
    <DropFiles disabled={loading} className="flex flex-col flex-1" onDrop={(files) => void uploadAll(files)}>
      <div className="flex flex-col flex-1">
        <input
          type="file"
          ref={uploaderRef}
          onChange={addToUpload}
          className="hidden"
          multiple={true}
          accept={type === 'image' ? 'image/*' : type === 'video' ? 'video/mp4' : 'image/*,video/mp4'}
          data-testid="media-upload-input"
        />
        <div className={clsx('flex items-center gap-[12px]', empty && !debouncedSearch && 'hidden')}>
          <div className="flex-1">
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t('search_media_by_name', 'Search by file name')}
              className="w-full h-[44px] px-[14px] rounded-[8px] bg-newBgColorInner border border-newColColor text-[14px] outline-none focus:border-[#612BD3]"
            />
          </div>
          <div className="flex gap-[8px]">{btn}</div>
        </div>
        <div className="w-full relative mt-[5px] mb-[5px] h-[46px]">
          {uploads.map((u) => (
            <div key={u.name} className="flex items-center gap-[12px] h-[46px] text-[14px]">
              <div className="flex-1 truncate">
                {t('uploading', 'Uploading')} {u.name}
              </div>
              <div className="w-[200px] h-[6px] rounded-full bg-newColColor overflow-hidden">
                <div className="h-full bg-[#612BD3] transition-all" style={{ width: `${Math.round(u.progress * 100)}%` }} />
              </div>
            </div>
          ))}
        </div>
        <div className={clsx('flex-1 relative', empty && 'bg-newTextColor/[0.02] rounded-[12px]')}>
          <div
            className={clsx(
              'absolute -left-[3px] -top-[3px] withp3 h-full overflow-x-hidden overflow-y-auto scrollbar scrollbar-thumb-newColColor scrollbar-track-newBgColorInner',
              empty && 'flex justify-center items-center gap-[20px] flex-col'
            )}
          >
            {empty && (
              <>
                <NoMediaIcon mode={storedMode()} />
                <div className="text-[20px] font-[600]">
                  {debouncedSearch
                    ? t('no_media_match_search', 'No media matches your search')
                    : t('you_dont_have_any_media_yet', "You don't have any media yet")}
                </div>
                <div className="whitespace-pre-line text-newTextColor/[0.6] text-center">
                  {t('select_or_upload_pictures_max_1gb', 'Select or upload pictures (maximum 1 GB per upload).')} {'\n'}
                  {t('you_can_drag_drop_pictures', 'You can also drag & drop pictures.')}
                </div>
                <div className="forceChange flex gap-[8px]">{btn}</div>
              </>
            )}
            {isLoading &&
              [...new Array(16)].map((_, i) => (
                <div className={clsx('px-[3px] py-[3px] float-left rounded-[6px] cursor-pointer w8-max aspect-square')} key={i}>
                  <div className="w-full h-full bg-newSep rounded-[6px] animate-pulse" />
                </div>
              ))}
            {results.map((media) => {
              const position = selected.findIndex((p) => p.id === media.id);
              return (
                <div
                  className={clsx('group px-[3px] py-[3px] float-left rounded-[6px] w8-max aspect-square', !standalone && 'cursor-pointer')}
                  key={media.id}
                  data-testid="media-item"
                >
                  <div
                    className={clsx(
                      'w-full h-full rounded-[6px] border-[4px] relative',
                      position >= 0 ? 'border-[#612BD3]' : 'border-transparent'
                    )}
                    onClick={addRemoveSelected(media)}
                  >
                    {position >= 0 ? (
                      <div className="text-white flex z-[101] justify-center items-center text-[14px] font-[500] w-[24px] h-[24px] rounded-full bg-[#612BD3] absolute -bottom-[10px] -end-[10px]">
                        {position + 1}
                      </div>
                    ) : (
                      <DeleteCircleIcon
                        className="cursor-pointer hidden z-[100] group-hover:block absolute -top-[5px] -end-[5px]"
                        onClick={deleteImage(media)}
                        data-testid="media-delete"
                      />
                    )}
                    <div className="absolute bottom-[10px] end-[10px] z-[100]">{media.name}</div>
                    <div className="w-full h-full rounded-[6px] overflow-hidden relative">
                      <div className="absolute z-[20] left-[50%] top-[50%] -translate-x-[50%] -translate-y-[50%]">
                        <div
                          onClick={maximize(media)}
                          className="cursor-pointer p-[4px] bg-black/40 hidden group-hover:block hover:scale-150 transition-all"
                        >
                          <svg width="30" height="30" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M2 9H0V14H5V12H2V9ZM0 5H2V2H5V0H0V5ZM12 12H9V14H14V9H12V12ZM9 0V2H12V5H14V0H9Z" fill="#F1F5F9" />
                          </svg>
                        </div>
                      </div>
                      {media.kind === 'video' ? (
                        <VideoFrame url={media.url} />
                      ) : (
                        <img width="100%" height="100%" className="w-full h-full object-cover" src={media.url} alt={media.alt || 'media'} />
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
        {(data?.pages || 0) > 1 && <Pagination current={page} totalPages={data!.pages} setPage={setPage} />}
        {!standalone && (
          <div className="flex justify-end mt-[32px] gap-[8px]">
            <button
              type="button"
              onClick={() => modals.closeCurrent()}
              className="cursor-pointer h-[52px] px-[20px] items-center justify-center border border-newTextColor/10 flex rounded-[10px]"
            >
              {t('cancel', 'Cancel')}
            </button>
            {!isLoading && !!data?.items?.length && (
              <button
                type="button"
                onClick={addMedia}
                disabled={selected.length === 0}
                className="cursor-pointer text-white disabled:opacity-80 disabled:cursor-not-allowed h-[52px] px-[20px] items-center justify-center bg-[#612BD3] flex rounded-[10px]"
              >
                {t('add_selected_media', 'Add selected media')}
              </button>
            )}
          </div>
        )}
      </div>
    </DropFiles>
  );
};

// showMediaBox opens the library as a modal and hands back what the person picks.
export function useMediaBox() {
  const modals = useModals();
  const t = useT();
  return useCallback(
    (onPick: (media: Media[]) => void, type?: 'image' | 'video') => {
      modals.openModal({
        title: t('media_library', 'Media Library'),
        askClose: false,
        fullScreen: true,
        size: 'calc(100% - 80px)',
        height: 'calc(100% - 80px)',
        children: <MediaBox setMedia={onPick} type={type} />,
      });
    },
    [modals, t]
  );
}
