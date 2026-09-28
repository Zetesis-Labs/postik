// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/tags.component.tsx y
// libraries/react-shared-libraries/src/form/color.picker.tsx (AGPL-3.0).
// Las etiquetas viajan por id, no por nombre, y se leen con TanStack Query.
import { FC, useCallback, useState } from 'react';
import clsx from 'clsx';
import { HexColorPicker } from 'react-colorful';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, Tag, tagsQuery } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useModals } from '@/components/ui/modals';
import { useToaster } from '@/components/ui/toaster';
import { CheckmarkIcon, DropdownArrowIcon, PlusIcon, TagIcon } from '@/components/ui/icons';
import { useClickOutside } from '@/components/launches/select-customer';

export type SelectedTag = { label: string; value: string };

export const TagsComponent: FC<{ initial: SelectedTag[]; onChange: (tags: SelectedTag[]) => void }> = (props) => {
  const tags = useQuery(tagsQuery);
  if (tags.isLoading) {
    return null;
  }
  return <TagsComponentInner {...props} allTags={tags.data ?? []} />;
};

const TagsComponentInner: FC<{ initial: SelectedTag[]; allTags: Tag[]; onChange: (tags: SelectedTag[]) => void }> = ({ initial, onChange, allTags }) => {
  const t = useT();
  const modals = useModals();
  const toaster = useToaster();
  const queryClient = useQueryClient();
  const [isOpen, setIsOpen] = useState(false);
  const [allowClose, setAllowClose] = useState(true);
  const [tagValue, setTagValue] = useState<Tag[]>(() =>
    initial.map((p) => allTags.find((a) => a.id === p.value)).filter((p): p is Tag => !!p)
  );

  const close = useCallback(() => {
    if (isOpen && allowClose) {
      setIsOpen(false);
    }
  }, [allowClose, isOpen]);
  const ref = useClickOutside<HTMLDivElement>(close);

  const select = useCallback(
    (modify: Tag[]) => {
      setTagValue(modify);
      onChange(modify.map((p) => ({ label: p.name, value: p.id })));
    },
    [onChange]
  );

  const addTag = useCallback(async () => {
    setAllowClose(false);
    const created: Tag | undefined = await new Promise((resolve) => {
      modals.openModal({
        title: t('add_new_tag', 'Add New Tag'),
        onClose: () => resolve(undefined),
        children: (closeModal) => <ShowModal close={closeModal} resolve={resolve} />,
      });
    });
    setTimeout(() => setAllowClose(true), 500);
    if (!created) {
      return;
    }
    await queryClient.invalidateQueries({ queryKey: tagsQuery.queryKey });
    select([...tagValue, created]);
  }, [modals, queryClient, select, t, tagValue]);

  const deleteTag = useCallback(
    async (tag: Tag, e: React.MouseEvent) => {
      setAllowClose(false);
      e.stopPropagation();
      const confirmed: boolean = await new Promise((resolve) => {
        modals.openModal({
          title: t('delete_tag', 'Delete Tag'),
          onClose: () => resolve(false),
          children: (closeModal) => <ConfirmDeleteModal tagName={tag.name} close={closeModal} resolve={resolve} />,
        });
      });
      if (!confirmed) {
        setTimeout(() => setAllowClose(true), 500);
        return;
      }
      const { error } = await api.DELETE('/tags/{id}', { params: { path: { id: tag.id } } });
      if (error) {
        toaster.show(t(error.code, error.message), 'warning');
      } else {
        const modify = tagValue.filter((a) => a.id !== tag.id);
        if (modify.length !== tagValue.length) {
          select(modify);
        }
        await queryClient.invalidateQueries({ queryKey: tagsQuery.queryKey });
      }
      setTimeout(() => setAllowClose(true), 500);
    },
    [modals, queryClient, select, t, tagValue, toaster]
  );

  return (
    <div
      ref={ref}
      className={clsx(
        'border rounded-[8px] justify-center flex items-center relative h-[44px] text-[15px] font-[600] select-none',
        isOpen ? 'border-[#612BD3]' : 'border-newTextColor/10'
      )}
      data-testid="post-tags"
    >
      <div onClick={() => setIsOpen(!isOpen)} className="px-[16px] justify-center flex gap-[8px] items-center h-full select-none flex-1">
        <div className="cursor-pointer">
          <TagIcon />
        </div>
        <div className="cursor-pointer flex gap-[4px]">
          {tagValue.length === 0 ? (
            t('add_new_tag', 'Add New Tag')
          ) : (
            <>
              <div className="h-full flex justify-center items-center px-[8px] rounded-[4px]" style={{ backgroundColor: tagValue[0].color }}>
                <span className="text-shadow-tags text-[#fff]">{tagValue[0].name}</span>
              </div>
              {tagValue.length > 1 ? <span>+{tagValue.length - 1}</span> : null}
            </>
          )}
        </div>
        <div className="cursor-pointer">
          <DropdownArrowIcon rotated={isOpen} />
        </div>
      </div>
      {isOpen && (
        <div className="z-[300] absolute start-0 bottom-[100%] w-[240px] bg-newBgColorInner p-[12px] menu-shadow -translate-y-[10px] flex flex-col">
          {allTags.map((p) => {
            const selected = tagValue.some((a) => a.id === p.id);
            return (
              <div
                onClick={() => select(selected ? tagValue.filter((a) => a.id !== p.id) : [...tagValue, p])}
                key={p.id}
                className="min-h-[40px] py-[8px] px-[20px] -mx-[12px] flex gap-[8px] items-center group cursor-pointer"
                role="checkbox"
                aria-checked={selected}
              >
                <Check value={selected} />
                <div className="h-full flex items-center flex-1 break-all">
                  <span className="text-[#fff] px-[8px] rounded-[8px] text-shadow-tags" style={{ backgroundColor: p.color }}>
                    {p.name}
                  </span>
                </div>
                {!selected && (
                  <div
                    onClick={(e) => deleteTag(p, e)}
                    className="ms-auto transition-opacity cursor-pointer text-red-500 text-[14px] font-[600]"
                    aria-label={t('delete_tag', 'Delete Tag')}
                  >
                    ×
                  </div>
                )}
              </div>
            );
          })}
          <div
            onClick={addTag}
            className="cursor-pointer gap-[8px] flex w-full h-[34px] rounded-[8px] mt-[12px] px-[16px] justify-center items-center bg-[#612BD3] text-white"
          >
            <div>
              <PlusIcon />
            </div>
            <div className="text-[13px] font-[600]">{t('add_new_tag', 'Add New Tag')}</div>
          </div>
        </div>
      )}
    </div>
  );
};

const Check: FC<{ value: boolean }> = ({ value }) => (
  <div
    className={clsx(
      'text-[10px] font-[500] text-center flex border border-btnSimple rounded-[6px] min-w-[20px] min-h-[20px] w-[20px] h-[20px] justify-center items-center',
      value && 'bg-[#612BD3]'
    )}
  >
    {value ? <CheckmarkIcon className="text-white" /> : ''}
  </div>
);

const ConfirmDeleteModal: FC<{ tagName: string; close: () => void; resolve: (value: boolean) => void }> = ({ tagName, close, resolve }) => {
  const t = useT();
  return (
    <div className="flex flex-col gap-[16px]">
      <p className="text-[14px]">{t('confirm_delete_tag', 'Are you sure you want to delete the tag "{{tagName}}"?', { tagName })}</p>
      <div className="flex gap-[8px] justify-end">
        <Button
          onClick={() => {
            resolve(false);
            close();
          }}
        >
          {t('cancel', 'Cancel')}
        </Button>
        <Button
          onClick={() => {
            resolve(true);
            close();
          }}
          className="bg-red-500 hover:bg-red-600"
        >
          {t('delete', 'Delete')}
        </Button>
      </div>
    </div>
  );
};

const ShowModal: FC<{ close: () => void; resolve: (value: Tag | undefined) => void }> = ({ close, resolve }) => {
  const t = useT();
  const toaster = useToaster();
  const [color, setColor] = useState('#942828');
  const [tagName, setTagName] = useState('');
  const save = useCallback(async () => {
    const { data, error } = await api.POST('/tags', { body: { name: tagName, color } });
    if (error) {
      toaster.show(t(error.code, error.message), 'warning');
      return;
    }
    resolve(data);
    close();
  }, [close, color, resolve, t, tagName, toaster]);
  return (
    <div>
      <Input name="name" disableForm={true} label={t('tag_name', 'Name')} value={tagName} onChange={(e) => setTagName(e.target.value)} />
      <div className="flex flex-col gap-[6px]">
        <div className="text-[14px]">{t('label_tag_color', 'Tag Color')}</div>
        <div className="flex items-end gap-[20px]">
          <div>
            <HexColorPicker color={color} onChange={setColor} />
          </div>
          <div className="flex gap-[10px]">
            <div>
              <div className="w-[20px] h-[20px]" style={{ backgroundColor: color }} />
            </div>
            <div>{color}</div>
          </div>
        </div>
      </div>
      <Button onClick={save} className="mt-[16px]" disabled={!tagName.trim()}>
        {t('save', 'Save')}
      </Button>
    </div>
  );
};
