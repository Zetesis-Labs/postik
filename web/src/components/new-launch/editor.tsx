// Adaptado de Postiz v2.24.0: apps/frontend/src/components/new-launch/editor.tsx,
// delay.component.tsx, add.post.button.tsx, bold.text.tsx, u.text.tsx,
// apps/frontend/src/components/launches/up.down.arrow.tsx,
// apps/frontend/src/components/launches/information.component.tsx y el
// MultiMediaComponent de apps/frontend/src/components/media/media.component.tsx (AGPL-3.0).
// Sin copiloto, firmas, menciones, IA ni «Diseñar medio» (funcional §9).
import { ClipboardEvent, FC, forwardRef, ReactNode, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from 'react';
import clsx from 'clsx';
import EmojiPicker, { Theme } from 'emoji-picker-react';
import { EditorContent, Extension, useEditor, type Editor as TiptapEditor } from '@tiptap/react';
import Document from '@tiptap/extension-document';
import Paragraph from '@tiptap/extension-paragraph';
import Text from '@tiptap/extension-text';
import Bold from '@tiptap/extension-bold';
import Underline from '@tiptap/extension-underline';
import { BulletList, ListItem } from '@tiptap/extension-list';
import { Placeholder, UndoRedo } from '@tiptap/extensions';
import { useT } from '@/lib/i18n';
import { useShallow } from '@/lib/store';
import { uploadFile } from '@/lib/upload';
import { plainText, characterLimit, countCharacters } from '@/lib/post-text';
import {
  CloseCircleIcon,
  ConnectionLineIcon,
  ChevronUpIcon,
  DelayIcon,
  EmojiIcon,
  InsertMediaIcon,
  LockIcon,
  ResetIcon,
  TrashIcon,
  VerticalDividerIcon,
} from '@/components/ui/icons';
import { deleteDialog } from '@/components/ui/modals';
import { useToaster } from '@/components/ui/toaster';
import { storedMode } from '@/components/layout/mode';
import { useMediaBox, VideoFrame } from '@/components/media/media-box';
import { useClickOutside } from '@/components/launches/select-customer';
import { makeId } from '@/components/new-launch/add-edit-modal';
import { useExistingData } from '@/components/new-launch/existing-data';
import { MediaItem, PostComment, useLaunchStore, Values } from '@/components/new-launch/store';

const MAX_UPLOAD_SIZE = 1024 * 1024 * 1024;

const InterceptBoldShortcut = Extension.create({
  name: 'preventBoldWithUnderline',
  addKeyboardShortcuts() {
    return {
      'Mod-b': () => {
        this?.editor?.commands?.unsetUnderline();
        return this?.editor?.commands?.toggleBold();
      },
    };
  },
});

const InterceptUnderlineShortcut = Extension.create({
  name: 'preventUnderlineWithUnderline',
  addKeyboardShortcuts() {
    return {
      'Mod-u': () => {
        this?.editor?.commands?.unsetBold();
        return this?.editor?.commands?.toggleUnderline();
      },
    };
  },
});

export const EditorWrapper: FC = () => {
  const t = useT();
  const {
    setGlobalValueText,
    setInternalValueText,
    addRemoveInternal,
    internal,
    global,
    current,
    addInternalValue,
    addGlobalValue,
    setInternalValueMedia,
    appendInternalValueMedia,
    appendGlobalValueMedia,
    setGlobalValueMedia,
    changeOrderGlobal,
    changeOrderInternal,
    deleteGlobalValue,
    deleteInternalValue,
    postComment,
    loadedState,
    setLoadedState,
    comments,
  } = useLaunchStore(
    useShallow((state) => ({
      internal: state.internal.find((p) => p.integration.id === state.current),
      global: state.global,
      comments: state.comments,
      current: state.current,
      addRemoveInternal: state.addRemoveInternal,
      setInternalValueText: state.setInternalValueText,
      setGlobalValueText: state.setGlobalValueText,
      addInternalValue: state.addInternalValue,
      addGlobalValue: state.addGlobalValue,
      setGlobalValueMedia: state.setGlobalValueMedia,
      setInternalValueMedia: state.setInternalValueMedia,
      changeOrderGlobal: state.changeOrderGlobal,
      changeOrderInternal: state.changeOrderInternal,
      deleteGlobalValue: state.deleteGlobalValue,
      deleteInternalValue: state.deleteInternalValue,
      appendInternalValueMedia: state.appendInternalValueMedia,
      appendGlobalValueMedia: state.appendGlobalValueMedia,
      postComment: state.postComment,
      loadedState: state.loaded,
      setLoadedState: state.setLoaded,
    }))
  );

  const existingData = useExistingData();
  const [loaded, setLoaded] = useState(true);

  useEffect(() => {
    if (loaded && loadedState) {
      return;
    }
    setLoadedState?.(true);
    setLoaded(true);
  }, [loaded, loadedState]);

  const canEdit = current === 'global' || !!internal;
  const items: Values[] = internal ? internal.integrationValue : global;

  const changeValue = useCallback(
    (index: number) => (value: string) => (internal ? setInternalValueText(current, index, value) : setGlobalValueText(index, value)),
    [current, internal, setGlobalValueText, setInternalValueText]
  );
  const changeImages = useCallback(
    (index: number) => (value: MediaItem[]) => (internal ? setInternalValueMedia(current, index, value) : setGlobalValueMedia(index, value)),
    [current, internal, setGlobalValueMedia, setInternalValueMedia]
  );
  const appendImages = useCallback(
    (index: number) => (value: MediaItem[]) =>
      internal ? appendInternalValueMedia(current, index, value) : appendGlobalValueMedia(index, value),
    [appendGlobalValueMedia, appendInternalValueMedia, current, internal]
  );
  const changeOrder = useCallback(
    (index: number) => (direction: 'up' | 'down') => {
      if (internal) {
        changeOrderInternal(current, index, direction);
      } else {
        changeOrderGlobal(index, direction);
      }
      setLoaded(false);
    },
    [changeOrderGlobal, changeOrderInternal, current, internal]
  );

  const goBackToGlobal = useCallback(async () => {
    if (
      await deleteDialog(
        t('are_you_sure_go_back_to_global_mode', 'This action is irreversible. Are you sure you want to go back to global mode?'),
        t('yes_go_back_to_global_mode', 'Yes, go back to global mode')
      )
    ) {
      setLoaded(false);
      addRemoveInternal(current);
    }
  }, [addRemoveInternal, current, t]);

  const addValue = useCallback(
    (index: number) => () => {
      setTimeout(() => {
        const content = document.querySelector('#social-content');
        content?.scrollTo({ top: content.scrollHeight });
      }, 20);
      const value = [{ delay: 0, content: '', id: makeId(), media: [] }];
      return internal ? addInternalValue(index, current, value) : addGlobalValue(index, value);
    },
    [addGlobalValue, addInternalValue, current, internal]
  );

  const deletePost = useCallback(
    (index: number) => async () => {
      if (!(await deleteDialog(t('are_you_sure_delete_this_post', 'Are you sure you want to delete this post?'), t('yes_delete_it', 'Yes, delete it!')))) {
        return;
      }
      if (internal) {
        deleteInternalValue(current, index);
      } else {
        deleteGlobalValue(index);
      }
      setLoaded(false);
    },
    [current, deleteGlobalValue, deleteInternalValue, internal, t]
  );

  if (!loaded || !loadedState) {
    return null;
  }

  return (
    <div
      className={clsx(
        'relative flex-col gap-[20px] flex-1',
        (items.length === 1 || !canEdit || !comments) && 'flex',
        (!canEdit || !comments) && 'bg-newSettings rounded-[12px]'
      )}
    >
      {!canEdit && (
        <>
          <div
            onClick={() => {
              setLoaded(false);
              addRemoveInternal(current);
            }}
            className="text-center absolute w-full h-full p-[20px] left-0 top-0 items-center justify-center flex z-[101] flex-col gap-[16px]"
          >
            <div>
              <div className="w-[54px] h-[54px] rounded-full absolute z-[101] flex justify-center items-center">
                <LockIcon />
              </div>
              <div className="w-[54px] h-[54px] rounded-full bg-newSettings opacity-80" />
            </div>
            <div className="text-[14px] font-[600] text-white">
              {t('click_to_exit_global_editing', 'Click this button to exit global editing and customize the post for this channel')}
            </div>
            <div>
              <div className="text-white rounded-[8px] h-[44px] px-[20px] bg-[#D82D7E] cursor-pointer flex justify-center items-center">
                {t('edit_content', 'Edit content')}
              </div>
            </div>
          </div>
          <div className="absolute w-full h-full left-0 top-0 bg-newBackdrop opacity-60 z-[100] rounded-[12px]" />
        </>
      )}
      {items.map((g, index) => (
        <div
          key={g.id}
          className={clsx(
            'relative flex flex-col gap-[20px] flex-1 bg-newSettings',
            index === 0 && 'rounded-t-[12px]',
            (index === items.length - 1 || !comments) && 'rounded-b-[12px]',
            !canEdit && 'blur-s',
            ((!canEdit && index > 0) || (!comments && index > 0)) && 'hidden'
          )}
          data-testid="post-value"
        >
          <div className="flex gap-[5px] flex-1 w-full">
            <div className="flex-1 flex w-full">
              {index > 0 && (
                <div className="flex justify-center pl-[12px] text-newSep">
                  <ConnectionLineIcon />
                </div>
              )}
              <Editor
                comments={comments}
                allValues={items}
                onChange={changeValue(index)}
                key={index}
                num={index}
                value={g.content}
                pictures={g.media}
                setImages={changeImages(index)}
                appendImages={appendImages(index)}
                childButton={
                  (canEdit && items.length - 1 === index) || !comments ? (
                    <div className="flex items-center">
                      <div className="flex-1">
                        {comments && <AddPostButton num={index} onClick={addValue(index)} postComment={postComment} />}
                      </div>
                      {!!internal && !existingData?.integration && (
                        <div className="mt-[12px] flex gap-[20px] items-center cursor-pointer select-none" onClick={goBackToGlobal}>
                          <div className="flex gap-[6px] items-center">
                            <div className="w-[8px] h-[8px] rounded-full bg-[#FC69FF]" />
                            <div className="text-[14px] font-[600]">{t('editing_a_specific_network', 'Editing a Specific Network')}</div>
                          </div>
                          <div className="flex gap-[6px] items-center">
                            <div>
                              <ResetIcon />
                            </div>
                            <div className="text-[13px] font-[600]">{t('back_to_global', 'Back to global')}</div>
                          </div>
                        </div>
                      )}
                    </div>
                  ) : null
                }
              />
            </div>
            {comments && (
              <div className="flex flex-col items-center gap-[10px] pe-[12px]">
                <UpDownArrow isUp={index !== 0} isDown={index !== items.length - 1} onChange={changeOrder(index)} />
                {items.length > 1 && (
                  <TrashIcon
                    onClick={deletePost(index)}
                    data-tooltip-id="tooltip"
                    data-tooltip-content={t('delete_post_tooltip', 'Delete Post')}
                    className="cursor-pointer text-[#FF3F3F]"
                  />
                )}
                {index > 0 && <DelayComponent currentIndex={index} currentDelay={g.delay} />}
              </div>
            )}
          </div>
        </div>
      ))}
    </div>
  );
};

export const Editor: FC<{
  value: string;
  num: number;
  pictures: MediaItem[];
  allValues: Values[];
  onChange: (value: string) => void;
  setImages: (value: MediaItem[]) => void;
  appendImages: (value: MediaItem[]) => void;
  comments: boolean | 'no-media';
  childButton?: ReactNode;
}> = (props) => {
  const { allValues, pictures, setImages, num, appendImages, childButton, comments } = props;
  const [id] = useState(makeId());
  const [emojiPickerOpen, setEmojiPickerOpen] = useState(false);
  const t = useT();
  const toaster = useToaster();
  const editorRef = useRef<{ editor: TiptapEditor | null } | null>(null);
  const [loading, setLoading] = useState(false);
  const [isDragActive, setDragActive] = useState(false);
  const [editorVersion, setEditorVersion] = useState(0);

  const upload = useCallback(
    async (files: File[]) => {
      if (!files.length) {
        return;
      }
      const totalSize = files.reduce((acc, file) => acc + file.size, 0);
      if (totalSize > MAX_UPLOAD_SIZE) {
        toaster.show(t('upload_size_limit_exceeded', 'Upload size limit exceeded. Maximum 1 GB per upload session.'), 'warning');
        return;
      }
      setLoading(true);
      const uploaded: MediaItem[] = [];
      for (const file of files) {
        const result = await uploadFile(file, () => {});
        if (result.ok) {
          uploaded.push({ id: result.media.id, path: result.media.url, kind: result.media.kind, alt: result.media.alt });
        } else {
          toaster.show(`${file.name}: ${t(result.code)}`, 'warning');
        }
      }
      setLoading(false);
      if (uploaded.length) {
        appendImages(uploaded);
      }
    },
    [appendImages, t, toaster]
  );

  const paste = useCallback(
    (event: ClipboardEvent) => {
      if (num > 0 && comments === 'no-media') {
        return;
      }
      const files: File[] = [];
      for (const item of Array.from(event.clipboardData?.items ?? [])) {
        if (item.kind === 'file') {
          const file = item.getAsFile();
          if (file) {
            files.push(file);
          }
        }
      }
      void upload(files);
    },
    [comments, num, upload]
  );

  const valueWithoutHtml = useMemo(() => plainText(props.value || ''), [props.value]);

  const addText = useCallback((emoji: string) => {
    editorRef.current?.editor?.commands?.insertContent(emoji);
    editorRef.current?.editor?.commands?.focus();
  }, []);

  const focusEnd = () => {
    if (editorRef.current?.editor?.isFocused) {
      return;
    }
    editorRef.current?.editor?.commands?.focus('end');
  };

  return (
    <div className="flex flex-col gap-[20px] flex-1">
      <div className={clsx('relative flex-1 px-[12px] pt-[12px] pb-[12px] flex flex-col', num > 0 && '!rounded-bs-[0]')} id={id}>
        <div className="relative cursor-text flex flex-1 flex-col">
          <div
            className="flex flex-1 flex-col"
            onDragOver={(e) => {
              e.preventDefault();
              setDragActive(true);
            }}
            onDragLeave={() => setDragActive(false)}
            onDrop={(e) => {
              e.preventDefault();
              setDragActive(false);
              if (loading) {
                toaster.show('Upload current in progress, please wait and then try again.', 'warning');
                return;
              }
              void upload(Array.from(e.dataTransfer.files));
            }}
          >
            <div
              className={clsx(
                'absolute left-0 top-0 w-full h-full bg-black/70 z-[300] transition-all items-center justify-center flex text-white text-sm',
                !isDragActive ? 'pointer-events-none opacity-0' : 'opacity-100'
              )}
            >
              {t('drop_files_here_to_upload', 'Drop your files here to upload')}
            </div>
            <div className="px-[10px] pt-[10px] bg-newBgColorInner rounded-t-[6px] relative z-[99]">
              <OnlyEditor value={props.value} onChange={props.onChange} paste={paste} ref={editorRef} onReady={() => setEditorVersion((v) => v + 1)} />
            </div>
            <div className="bg-newBgColorInner flex-1" onClick={focusEnd} />
            <div className="w-full h-[46px] bg-newBgColorInner cursor-text flex items-center px-[12px] text-[12px] text-newTextColor/60" onClick={focusEnd}>
              {loading && t('uploading', 'Uploading')}
            </div>
            <div className="flex bg-newBgColorInner rounded-b-[6px] cursor-default">
              <MultiMediaComponent
                key={editorVersion}
                mediaNotAvailable={num > 0 && comments === 'no-media'}
                value={pictures}
                onChange={setImages}
                information={<InformationComponent isPicture={pictures?.length > 0} text={valueWithoutHtml} />}
                toolBar={
                  <div className="flex gap-[5px]">
                    <UText editor={editorRef.current?.editor ?? null} />
                    <BoldText editor={editorRef.current?.editor ?? null} />
                    <div
                      data-tooltip-id="tooltip"
                      data-tooltip-content={t('insert_emoji', 'Insert Emoji')}
                      className="select-none cursor-pointer rounded-[6px] w-[30px] h-[30px] bg-newColColor flex justify-center items-center"
                      onClick={() => setEmojiPickerOpen(!emojiPickerOpen)}
                    >
                      <EmojiIcon />
                    </div>
                    <div className="relative">
                      <div className={clsx('absolute z-[500] -start-[50px]', num === 0 && allValues?.length > 1 ? 'top-[35px]' : 'bottom-[35px]')}>
                        <EmojiPicker
                          height={400}
                          theme={storedMode() === 'light' ? Theme.LIGHT : Theme.DARK}
                          onEmojiClick={(e) => {
                            addText(e.emoji);
                            setEmojiPickerOpen(false);
                          }}
                          open={emojiPickerOpen}
                        />
                      </div>
                    </div>
                  </div>
                }
              />
            </div>
            <div>{childButton}</div>
          </div>
        </div>
      </div>
    </div>
  );
};

export const OnlyEditor = forwardRef<
  { editor: TiptapEditor | null },
  { value: string; onChange: (value: string) => void; paste?: (event: ClipboardEvent) => void; onReady?: () => void }
>(({ value, onChange, paste, onReady }, ref) => {
  const t = useT();
  const editor = useEditor({
    extensions: [
      Document,
      Paragraph,
      Text,
      Underline,
      Bold,
      InterceptBoldShortcut,
      InterceptUnderlineShortcut,
      BulletList,
      ListItem,
      Placeholder.configure({ placeholder: t('write_something', 'Write something …'), emptyEditorClass: 'is-editor-empty' }),
      UndoRedo.configure({ depth: 100, newGroupDelay: 100 }),
    ],
    content: value || '',
    shouldRerenderOnTransaction: true,
    immediatelyRender: false,
    editorProps: {
      handlePaste: (_view, event) => {
        paste?.(event as unknown as ClipboardEvent);
        return false;
      },
    },
    onCreate: () => onReady?.(),
    onUpdate: (innerProps) => {
      onChange?.(innerProps.editor.getHTML());
    },
  });
  useImperativeHandle(ref, () => ({ editor }), [editor]);
  return <EditorContent editor={editor} data-testid="post-editor-input" />;
});

const BoldText: FC<{ editor: TiptapEditor | null }> = ({ editor }) => (
  <div
    data-tooltip-id="tooltip"
    data-tooltip-content="Bold Text"
    onClick={() => {
      editor?.commands?.unsetUnderline();
      editor?.commands?.toggleBold();
      editor?.commands?.focus();
    }}
    className="select-none cursor-pointer rounded-[6px] w-[30px] h-[30px] bg-newColColor flex justify-center items-center"
  >
    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16" fill="none">
      <path
        d="M4 8.00033H9.33333C10.8061 8.00033 12 6.80642 12 5.33366C12 3.8609 10.8061 2.66699 9.33333 2.66699H4V8.00033ZM4 8.00033H10C11.4728 8.00033 12.6667 9.19423 12.6667 10.667C12.6667 12.1398 11.4728 13.3337 10 13.3337H4V8.00033Z"
        stroke="currentColor"
        strokeWidth="1.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  </div>
);

const UText: FC<{ editor: TiptapEditor | null }> = ({ editor }) => (
  <div
    data-tooltip-id="tooltip"
    data-tooltip-content="Underline"
    onClick={() => {
      editor?.commands?.unsetBold();
      editor?.commands?.toggleUnderline();
      editor?.commands?.focus();
    }}
    className="select-none cursor-pointer rounded-[6px] w-[30px] h-[30px] bg-newColColor flex justify-center items-center"
  >
    <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16" fill="none">
      <path
        d="M11.9993 2.66699V7.33366C11.9993 9.5428 10.2085 11.3337 7.99935 11.3337C5.79021 11.3337 3.99935 9.5428 3.99935 7.33366V2.66699M2.66602 14.0003H13.3327"
        stroke="currentColor"
        strokeWidth="1.2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  </div>
);

const AddPostButton: FC<{ onClick: () => void; num: number; postComment: PostComment }> = ({ onClick, postComment }) => {
  const t = useT();
  const [key, fallback] =
    postComment === PostComment.ALL
      ? ['add_comment_or_post', 'Add comment or post']
      : postComment === PostComment.POST
        ? ['add_post', 'Add post']
        : ['add_comment', 'Add comment'];
  return (
    <div className="flex">
      <div
        onClick={onClick}
        className="select-none cursor-pointer h-[34px] rounded-[6px] flex bg-[#D82D7E] gap-[8px] justify-center items-center pl-[16px] pr-[20px] text-[13px] font-[600] mt-[12px]"
      >
        <div>
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16" fill="none">
            <path d="M8.00065 3.33301V12.6663M3.33398 7.99967H12.6673" stroke="white" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </div>
        <div className="!text-white">{t(key, fallback)}</div>
      </div>
    </div>
  );
};

const UpDownArrow: FC<{ isUp: boolean; isDown: boolean; onChange: (type: 'up' | 'down') => void }> = ({ isUp, isDown, onChange }) => (
  <div className="flex flex-col gap-[8px] pt-[8px]">
    <button
      type="button"
      onClick={() => onChange('up')}
      className={clsx('outline-none w-[20px] h-[20px] flex justify-center items-center', isUp ? 'cursor-pointer' : 'pointer-events-none text-textColor opacity-50')}
    >
      <ChevronUpIcon />
    </button>
    <button
      type="button"
      onClick={() => onChange('down')}
      className={clsx(
        'outline-none rounded-bl-[20px] w-[20px] h-[20px] flex justify-center items-center',
        isDown ? 'cursor-pointer' : 'pointer-events-none text-textColor opacity-50'
      )}
    >
      <ChevronUpIcon style={{ transform: 'rotate(180deg)' }} />
    </button>
  </div>
);

const delayOptions = [
  { value: 1, label: '1m' },
  { value: 2, label: '2m' },
  { value: 5, label: '5m' },
  { value: 10, label: '10m' },
  { value: 15, label: '15m' },
  { value: 30, label: '30m' },
  { value: 60, label: '1h' },
  { value: 120, label: '2h' },
];

const DelayComponent: FC<{ currentIndex: number; currentDelay: number }> = ({ currentIndex, currentDelay }) => {
  const t = useT();
  const [isOpen, setIsOpen] = useState(false);
  const [customValue, setCustomValue] = useState('');
  const isCustomDelay = currentDelay > 0 && !delayOptions.some((opt) => opt.value === currentDelay);
  useEffect(() => {
    if (isOpen) {
      setCustomValue(isCustomDelay ? String(currentDelay) : '');
    }
  }, [isOpen, isCustomDelay, currentDelay]);
  const { current, setInternalDelay, setGlobalDelay } = useLaunchStore(
    useShallow((state) => ({ current: state.current, setGlobalDelay: state.setGlobalDelay, setInternalDelay: state.setInternalDelay }))
  );
  const close = useCallback(() => setIsOpen(false), []);
  const ref = useClickOutside<HTMLDivElement>(close);
  const handleSelectDelay = (minutes: number) => {
    if (current !== 'global') {
      setInternalDelay(current, currentIndex, minutes);
    } else {
      setGlobalDelay(currentIndex, minutes);
    }
    setIsOpen(false);
  };
  const label = !currentDelay ? null : delayOptions.find((opt) => opt.value === currentDelay)?.label || `${currentDelay} min`;
  return (
    <div ref={ref} className="relative">
      <div
        onClick={() => setIsOpen(!isOpen)}
        data-tooltip-id="tooltip"
        data-tooltip-content={!currentDelay ? t('delay_comment', 'Delay comment') : `${t('delay_comment_by', 'Comment delayed by')} ${label}`}
        className={clsx('cursor-pointer flex items-center gap-[4px]', currentDelay > 0 && 'bg-[#D82D7E] text-white rounded-full')}
      >
        <DelayIcon />
      </div>
      {isOpen && (
        <div className="z-[300] absolute end-0 top-[100%] w-[200px] bg-newBgColorInner p-[8px] menu-shadow translate-y-[10px] flex flex-col rounded-[8px]">
          <div className="grid grid-cols-4 gap-[4px]">
            {delayOptions.map((option) => (
              <div
                onClick={() => handleSelectDelay(option.value)}
                key={option.value}
                className={clsx(
                  'h-[32px] flex items-center justify-center rounded-[4px] cursor-pointer hover:bg-newBgColor text-[13px]',
                  currentDelay === option.value && 'bg-[#612BD3] text-white hover:bg-[#612BD3]'
                )}
              >
                {option.label}
              </div>
            ))}
          </div>
          <div className="border-t border-newTextColor/10 mt-[8px] pt-[8px]">
            <div className="flex gap-[4px]">
              <input
                type="number"
                min="1"
                value={customValue}
                onChange={(e) => setCustomValue(e.target.value)}
                onClick={(e) => e.stopPropagation()}
                placeholder="Custom min"
                className={clsx(
                  'flex-1 w-full h-[32px] px-[8px] rounded-[4px] bg-newBgColor border text-[13px] outline-none focus:border-[#612BD3]',
                  isCustomDelay ? 'border-[#612BD3]' : 'border-newTextColor/10'
                )}
              />
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  const value = parseInt(customValue, 10);
                  if (value > 0) {
                    handleSelectDelay(value);
                    setCustomValue('');
                  }
                }}
                className="h-[32px] px-[10px] rounded-[4px] bg-[#612BD3] text-white text-[12px] font-[600] hover:bg-[#612BD3]/80"
              >
                Set
              </button>
            </div>
          </div>
          {currentDelay > 0 && (
            <button type="button" onClick={() => handleSelectDelay(0)} className="mt-[8px] h-[32px] w-full rounded-[4px] text-[13px] text-red-400 hover:bg-red-400/10">
              Remove delay
            </button>
          )}
        </div>
      )}
    </div>
  );
};

const Valid: FC = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16" fill="none">
    <path
      d="M6 7.33333L8 9.33333L14.6667 2.66667M10.6667 2H5.2C4.0799 2 3.51984 2 3.09202 2.21799C2.71569 2.40973 2.40973 2.71569 2.21799 3.09202C2 3.51984 2 4.07989 2 5.2V10.8C2 11.9201 2 12.4802 2.21799 12.908C2.40973 13.2843 2.71569 13.5903 3.09202 13.782C3.51984 14 4.07989 14 5.2 14H10.8C11.9201 14 12.4802 14 12.908 13.782C13.2843 13.5903 13.5903 13.2843 13.782 12.908C14 12.4802 14 11.9201 14 10.8V8"
      stroke="#00EB75"
      strokeWidth="1.2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
);

const Invalid: FC = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16" fill="none">
    <path
      d="M8.00049 6.00015V8.66682M8.00049 11.3335H8.00715M7.07737 2.59464L1.59411 12.0657C1.28997 12.591 1.1379 12.8537 1.16038 13.0693C1.17998 13.2573 1.2785 13.4282 1.4314 13.5394C1.60671 13.6668 1.91022 13.6668 2.51723 13.6668H13.4837C14.0908 13.6668 14.3943 13.6668 14.5696 13.5394C14.7225 13.4282 14.821 13.2573 14.8406 13.0693C14.8631 12.8537 14.711 12.591 14.4069 12.0657L8.92361 2.59463C8.62056 2.07119 8.46904 1.80947 8.27135 1.72157C8.09892 1.64489 7.90206 1.64489 7.72962 1.72157C7.53193 1.80947 7.38041 2.07119 7.07737 2.59464Z"
      stroke="white"
      strokeWidth="1.2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
);

// InformationComponent shows the characters used against the limit of the
// current channel or, in global mode, of the tightest selected channel.
const InformationComponent: FC<{ isPicture: boolean; text: string }> = ({ isPicture, text }) => {
  const t = useT();
  const { isGlobal, selectedIntegrations, internal, currentIntegration } = useLaunchStore(
    useShallow((state) => ({
      isGlobal: state.current === 'global',
      selectedIntegrations: state.selectedIntegrations,
      internal: state.internal,
      currentIntegration: state.integrations.find((p) => p.id === state.current),
    }))
  );
  const count = countCharacters(text);
  const limits = isGlobal
    ? selectedIntegrations
        .filter((p) => !internal.some((i) => i.integration.id === p.integration.id))
        .map((p) => characterLimit(p.integration.identifier))
        .filter((limit) => limit > 0)
    : [characterLimit(currentIntegration?.identifier ?? '')].filter((limit) => limit > 0);
  const limit = limits.length ? Math.min(...limits) : 0;
  const isValid = (isPicture || count > 0) && (!limit || count <= limit);
  return (
    <div
      className={clsx(
        'group rounded-[6px] gap-[4px] h-[30px] px-[6px] flex justify-center items-center relative',
        isValid ? 'border border-newColColor' : 'bg-[#FF3F3F]'
      )}
      data-testid="characters"
    >
      {isValid ? <Valid /> : <Invalid />}
      {limit > 0 && (
        <div className={clsx('text-[10px] font-[600] flex justify-center items-center', !isValid && 'text-white')}>
          {count}/{limit}
        </div>
      )}
      {!isPicture && !count && (
        <div className="z-[300] hidden rounded-[12px] bg-newBgColorInner group-hover:flex absolute end-0 bottom-[100%] mb-[5px] p-[12px] flex-col border border-[#FF3F3F]">
          <div className="text-sm text-[#FF3F3F] whitespace-nowrap">
            {t('your_post_should_have_at_least_one_character_or_one_image', 'Your post should have at least one character or one image.')}
          </div>
        </div>
      )}
    </div>
  );
};

const MultiMediaComponent: FC<{
  mediaNotAvailable?: boolean;
  value?: MediaItem[];
  onChange: (value: MediaItem[]) => void;
  toolBar?: ReactNode;
  information?: ReactNode;
}> = ({ onChange, value, toolBar, information, mediaNotAvailable }) => {
  const t = useT();
  const showMediaBox = useMediaBox();
  const currentMedia = value ?? [];

  const showModal = useCallback(() => {
    showMediaBox((picked) => {
      onChange([...currentMedia, ...picked.map((m) => ({ id: m.id, path: m.url, kind: m.kind, alt: m.alt }))]);
    });
  }, [currentMedia, onChange, showMediaBox]);

  const clearMedia = (topIndex: number) => () => onChange(currentMedia.filter((_, index) => index !== topIndex));

  return (
    <>
      <div className="b1 flex flex-col gap-[8px] rounded-bl-[8px] select-none w-full">
        <div className="flex gap-[10px] px-[12px]">
          {currentMedia.map((media, index) => (
            <div
              key={media.id}
              className="cursor-pointer rounded-[5px] w-[40px] h-[40px] border-2 border-tableBorder relative flex transition-all"
              data-testid="post-media"
            >
              <div className="w-full h-full relative group">
                {media.kind === 'video' ? (
                  <VideoFrame url={media.path} />
                ) : (
                  <img className="w-full h-full object-cover rounded-[4px]" src={media.path} alt={media.alt || ''} />
                )}
              </div>
              <CloseCircleIcon onClick={clearMedia(index)} className="absolute -end-[4px] -top-[4px] z-[20] rounded-full bg-white" />
            </div>
          ))}
        </div>
        <div className="flex gap-[8px] px-[12px] border-t border-newColColor w-full b1 text-textColor">
          {!mediaNotAvailable && (
            <div className="flex py-[10px] b2 items-center gap-[4px]">
              <div onClick={showModal} className="cursor-pointer h-[30px] rounded-[6px] justify-center items-center flex bg-newColColor px-[8px]" data-testid="insert-media">
                <div className="flex gap-[8px] items-center">
                  <div>
                    <InsertMediaIcon />
                  </div>
                  <div className="text-[10px] font-[600] maxMedia:hidden block">{t('insert_media', 'Insert Media')}</div>
                </div>
              </div>
            </div>
          )}
          {!mediaNotAvailable && (
            <div className="text-newColColor h-full flex items-center">
              <VerticalDividerIcon />
            </div>
          )}
          {!!toolBar && <div className="flex py-[10px] b2 items-center gap-[4px]">{toolBar}</div>}
          {information && <div className="flex-1 justify-end flex py-[10px] b2 items-center gap-[4px]">{information}</div>}
        </div>
      </div>
    </>
  );
};
