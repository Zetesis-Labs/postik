// Adaptado de Postiz v2.24.0: apps/frontend/src/components/new-launch/manage.modal.tsx (AGPL-3.0).
// Sin copiloto, acortador de enlaces, «Repetir», sets ni modo de salida de código (funcional §9).
import { FC, useCallback, useEffect, useMemo, useState } from 'react';
import clsx from 'clsx';
import { api, InvalidPost } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { newDayjs } from '@/lib/dates';
import { useShallow } from '@/lib/store';
import { Button } from '@/components/ui/button';
import { deleteDialog, useModals } from '@/components/ui/modals';
import { useToaster } from '@/components/ui/toaster';
import { ChevronDownIcon, CloseIcon, DropdownArrowSmallIcon, SettingsIcon, TrashIcon } from '@/components/ui/icons';
import { SelectCustomer } from '@/components/launches/select-customer';
import { TagsComponent } from '@/components/launches/tags';
import { AddEditModalProps } from '@/components/new-launch/add-edit-modal';
import { useExistingData } from '@/components/new-launch/existing-data';
import { PicksSocialsComponent } from '@/components/new-launch/picks-socials';
import { SelectCurrent } from '@/components/new-launch/select-current';
import { EditorWrapper } from '@/components/new-launch/editor';
import { ShowAllProviders } from '@/components/new-launch/preview';
import { DatePicker } from '@/components/new-launch/date-picker';
import { useLaunchStore, Values } from '@/components/new-launch/store';

type ScheduleType = 'draft' | 'now' | 'schedule' | 'update';

const toValues = (values: Values[]) =>
  values.map((v) => ({
    content: v.content,
    delayMinutes: v.delay || 0,
    media: (v.media || []).map((m) => ({ id: m.id })),
  }));

export const ManageModal: FC<AddEditModalProps> = (props) => {
  const t = useT();
  const existingData = useExistingData();
  const [loading, setLoading] = useState(false);
  const toaster = useToaster();
  const modal = useModals();
  const [showSettings, setShowSettings] = useState(false);
  const { mutate } = props;

  const { selectedIntegrations, hide, date, setDate, tags, setTags, integrations, setSelectedIntegrations, current, setHide, setCurrent, global, internal } =
    useLaunchStore(
      useShallow((state) => ({
        hide: state.hide,
        setHide: state.setHide,
        date: state.date,
        setDate: state.setDate,
        current: state.current,
        setCurrent: state.setCurrent,
        tags: state.tags,
        setTags: state.setTags,
        selectedIntegrations: state.selectedIntegrations,
        integrations: state.integrations,
        setSelectedIntegrations: state.setSelectedIntegrations,
        global: state.global,
        internal: state.internal,
      }))
    );

  useEffect(() => {
    if (hide) {
      setHide(false);
    }
  }, [hide]);

  const currentIntegrationText = useMemo(() => {
    if (current === 'global') {
      return (
        <div className="flex items-center gap-[10px]">
          <div className="relative">
            <SettingsIcon size={15} className="text-white" />
          </div>
          <div>Settings</div>
        </div>
      );
    }
    const currentIntegration = integrations.find((p) => p.id === current);
    return (
      <div className="flex items-center gap-[10px]">
        <div className="relative">
          <img src={`/icons/platforms/${currentIntegration?.identifier}.png`} className="w-[20px] h-[20px] rounded-[4px]" alt="" />
          <SettingsIcon size={15} className="text-white absolute -end-[5px] -bottom-[5px]" />
        </div>
        <div>
          {currentIntegration?.name} {t('channel_settings', 'Settings')}
        </div>
      </div>
    );
  }, [current, integrations, t]);

  const changeCustomer = useCallback(
    (customer: string) => {
      setSelectedIntegrations(
        integrations
          .filter((p) => p?.customer?.id === customer && !p.disabled && !p.inBetweenSteps)
          .map((p) => ({ settings: {}, selectedIntegrations: p }))
      );
    },
    [integrations, setSelectedIntegrations]
  );

  const askClose = useCallback(async () => {
    if (
      await deleteDialog(
        t('are_you_sure_you_want_to_close_this_modal_all_data_will_be_lost', 'Are you sure you want to close this modal? (all data will be lost)'),
        t('yes_close_it', 'Yes, close it!')
      )
    ) {
      modal.closeAll();
    }
  }, [modal, t]);

  const deletePost = useCallback(async () => {
    setLoading(true);
    if (!(await deleteDialog(t('are_you_sure_you_want_to_delete_post', 'Are you sure you want to delete this post?'), t('yes_delete_it', 'Yes, delete it!')))) {
      setLoading(false);
      return;
    }
    if (existingData.post) {
      await api.DELETE('/posts/{id}/group', { params: { path: { id: existingData.post.id } } });
    }
    mutate();
    modal.closeAll();
  }, [existingData, modal, mutate, t]);

  const showProblems = useCallback(
    (problems: InvalidPost['problems']) => {
      const problem = problems[0];
      if (!problem) {
        return;
      }
      const channel = integrations.find((i) => i.id === problem.channelId);
      toaster.show(`${channel ? `${channel.name}: ` : ''}${t(`post_problem_${problem.code}`)}`, 'warning');
      if (channel && selectedIntegrations.some((s) => s.integration.id === channel.id)) {
        setCurrent(channel.id);
        setHide(true);
      }
    },
    [integrations, selectedIntegrations, setCurrent, setHide, t, toaster]
  );

  const schedule = useCallback(
    (type: ScheduleType) => async () => {
      let republish = false;
      const existing = existingData.post;
      if (
        existing &&
        (type === 'now' || type === 'schedule') &&
        (existing.status === 'published' || (existing.status === 'scheduled' && newDayjs().isAfter(date)))
      ) {
        const whatToDo = await new Promise<'update' | 'republish' | 'cancel'>((resolve) => {
          modal.openModal({
            title: t('what_do_you_want_to_do', 'What do you want to do?'),
            onClose: () => resolve('cancel'),
            children: (
              <div className="flex flex-col">
                <div className="text-[20px] mb-[20px]">
                  {t('post_already_published_republish_warning', 'This post was already published. Republishing will publish it again to')}{' '}
                  {selectedIntegrations.map((p) => p.integration.name).join(', ')} {t('republish_at', 'at')} {date.format('DD/MM/YYYY HH:mm')}.
                </div>
                <div className="flex w-full gap-[10px]">
                  <div className="flex-1 flex">
                    <Button type="button" className="flex-1" onClick={() => { modal.closeCurrent(); resolve('update'); }}>
                      {t('just_update_post_details', 'Just update the post details')}
                    </Button>
                  </div>
                  <div className="flex-1 flex">
                    <Button type="button" className="flex-1" onClick={() => { modal.closeCurrent(); resolve('republish'); }}>
                      {t('republish_the_post', 'Republish the post')}
                    </Button>
                  </div>
                </div>
              </div>
            ),
          });
        });
        if (whatToDo === 'cancel') {
          return;
        }
        if (whatToDo === 'update') {
          type = 'update';
        } else {
          republish = true;
        }
      }

      setLoading(true);
      const valuesFor = (id: string) => internal.find((p) => p.integration.id === id)?.integrationValue ?? global;
      const tagIds = tags.map((tag) => tag.value);

      if (existing) {
        const publishAt = type === 'now' ? newDayjs() : date;
        const { data, error, response } = await api.PUT('/posts/{id}', {
          params: { path: { id: existing.id } },
          body: {
            mode: type === 'update' || type === 'draft' ? 'update' : 'schedule',
            republish,
            publishAt: publishAt.utc().format(),
            values: toValues(valuesFor(existing.channel.id)),
            settings: selectedIntegrations[0]?.settings ?? {},
            tags: tagIds,
          },
        });
        setLoading(false);
        if (!data) {
          if (response.status === 400 && error && 'problems' in error) {
            showProblems(error.problems);
          } else {
            toaster.show(t('something_went_wrong', 'Something went wrong. Please try again.'), 'warning');
          }
          return;
        }
        mutate();
        toaster.show(t('updated_successfully', 'Updated successfully'));
        modal.closeAll();
        return;
      }

      const { data, error, response } = await api.POST('/posts', {
        body: {
          type: type === 'update' ? 'schedule' : type,
          publishAt: date.utc().format(),
          tags: tagIds,
          posts: selectedIntegrations.map((p) => ({
            channelId: p.integration.id,
            values: toValues(valuesFor(p.integration.id)),
            settings: p.settings ?? {},
          })),
        },
      });
      setLoading(false);
      if (!data) {
        if (response.status === 400 && error && 'problems' in error) {
          showProblems(error.problems);
        } else {
          toaster.show(t('something_went_wrong', 'Something went wrong. Please try again.'), 'warning');
        }
        return;
      }
      mutate();
      toaster.show(t('added_successfully', 'Added successfully'));
      modal.closeAll();
    },
    [date, existingData, global, internal, modal, mutate, selectedIntegrations, showProblems, t, tags, toaster]
  );

  const disabled = selectedIntegrations.length === 0 || loading;
  return (
    <div className="w-full h-full flex-1 p-[40px] flex relative" data-testid="post-editor">
      <div className="flex flex-1 bg-newBgColorInner rounded-[20px] flex-col">
        <div className="flex-1 flex">
          <div className="flex flex-col flex-1 border-e border-newBorder">
            <div className="bg-newBgColor h-[65px] rounded-s-[20px] !rounded-b-[0] flex items-center gap-[12px] px-[20px] text-[20px] font-[600]">
              {t('create_post_title', 'Create Post')}
            </div>
            <div className="flex-1 flex flex-col gap-[16px]">
              <div className={clsx('flex-1 relative', showSettings && 'hidden')}>
                <div
                  id="social-content"
                  className="gap-[32px] flex flex-col pe-[8px] pt-[20px] ps-[20px] absolute top-0 left-0 w-full h-full overflow-x-hidden overflow-y-scroll scrollbar scrollbar-thumb-newColColor scrollbar-track-newBgColorInner"
                >
                  <div className="flex w-full">
                    <div className="flex flex-1">
                      <PicksSocialsComponent toolTip={true} />
                    </div>
                    <div>
                      <SelectCustomer onChange={changeCustomer} integrations={integrations} onSelected={() => setCurrent('global')} />
                    </div>
                  </div>
                  <div className="flex flex-1 gap-[6px] flex-col">
                    <div>{!existingData.integration && <SelectCurrent />}</div>
                    <div className="flex-1 flex">{!hide && <EditorWrapper />}</div>
                    <div id="social-empty" className={clsx('pb-[16px]')} />
                  </div>
                </div>
              </div>
              <div
                id="wrapper-settings"
                className={clsx('pb-[20px] px-[20px] select-none', showSettings && 'flex-1 flex pt-[20px]', current === 'global' && 'hidden')}
              >
                <div className="flex-1 flex flex-col rounded-[12px] gap-[12px] overflow-hidden bg-newSettings">
                  <div
                    onClick={() => setShowSettings(!showSettings)}
                    className={clsx('bg-[#612BD3] rounded-[12px] flex items-center gap-[8px] cursor-pointer p-[12px]', showSettings ? '!rounded-b-none' : '')}
                  >
                    <div className="flex-1 text-[14px] font-[600] text-white">{currentIntegrationText}</div>
                    <div>
                      <ChevronDownIcon rotated={showSettings} className="text-white" />
                    </div>
                  </div>
                  <div className={clsx(!showSettings ? 'hidden' : 'flex-1', 'text-[14px] text-textColor font-[500] relative')}>
                    <div className="absolute left-0 top-0 w-full h-full flex flex-col overflow-x-hidden overflow-y-auto scrollbar scrollbar-thumb-newBgColorInner scrollbar-track-newColColor">
                      <div id="social-settings" className="flex flex-col gap-[20px] bg-newBgColor p-[20px]">
                        {t('no_settings_for_channel', 'This channel has no extra settings.')}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div className="w-[580px] flex flex-col">
            <div className="bg-newBgColor h-[65px] rounded-e-[20px] !rounded-b-[0] flex items-center px-[20px] text-[20px] font-[600]">
              <div className="flex-1">{t('post_preview', 'Post Preview')}</div>
              <div className="cursor-pointer">
                <CloseIcon onClick={askClose} className="text-[#A3A3A3]" aria-label={t('close', 'Close')} />
              </div>
            </div>
            <div className="flex-1 relative">
              <div className="absolute top-0 p-[20px] pe-[8px] left-0 w-full h-full overflow-x-hidden overflow-y-scroll scrollbar scrollbar-thumb-newColColor scrollbar-track-newBgColorInner">
                <ShowAllProviders />
              </div>
            </div>
          </div>
        </div>
        <div className="select-none h-[84px] py-[20px] border-t border-newBorder flex items-center">
          <div className="flex-1 flex ps-[20px] gap-[8px]">
            <TagsComponent initial={tags} onChange={setTags} />
          </div>
          <div className="pe-[20px] flex items-center justify-end gap-[8px]">
            {existingData?.integration && (
              <button type="button" onClick={deletePost} className="cursor-pointer flex text-[#FF3F3F] gap-[8px] items-center text-[15px] font-[600]">
                <div>
                  <TrashIcon />
                </div>
                <div>{t('delete_post', 'Delete Post')}</div>
              </button>
            )}
            <DatePicker onChange={setDate} date={date} />
            <button
              type="button"
              disabled={disabled}
              onClick={schedule(existingData.post ? 'update' : 'draft')}
              className="relative cursor-pointer disabled:cursor-not-allowed px-[20px] h-[44px] bg-btnSimple justify-center items-center flex rounded-[8px] text-[15px] font-[600]"
            >
              {loading && (
                <div className="absolute left-[50%] top-[50%] -translate-y-[50%] -translate-x-[50%]">
                  <div className="animate-spin h-[20px] w-[20px] border-4 border-textColor border-t-transparent rounded-full" />
                </div>
              )}
              <div className={clsx(loading && 'invisible')}>
                {existingData.post ? t('update_details', 'Update details') : t('save_as_draft', 'Save as Draft')}
              </div>
            </button>
            <div className="group cursor-pointer relative">
              <button
                type="button"
                disabled={disabled}
                onClick={schedule('schedule')}
                className="text-white relative min-w-[180px] btnSub disabled:cursor-not-allowed disabled:opacity-80 outline-none gap-[8px] flex justify-center items-center h-[44px] rounded-[8px] bg-[#612BD3] ps-[20px] pe-[16px]"
              >
                {loading && (
                  <div className="absolute left-[50%] top-[50%] -translate-y-[50%] -translate-x-[50%]">
                    <div className="animate-spin h-[20px] w-[20px] border-4 border-white border-t-transparent rounded-full" />
                  </div>
                )}
                <div className={clsx('text-[15px] font-[600]', loading && 'invisible')}>
                  {selectedIntegrations.length === 0
                    ? t('check_circles_above', 'Check the circles above')
                    : !existingData?.post
                      ? t('add_to_calendar', 'Add to calendar')
                      : existingData.post.status === 'draft'
                        ? t('schedule', 'Schedule')
                        : t('update', 'Update')}
                </div>
                <div className="flex justify-center items-center h-[20px] w-[20px] pt-[4px] arrow-change">
                  <DropdownArrowSmallIcon className="group-hover:rotate-180 text-white" />
                </div>
              </button>
              <button
                type="button"
                onClick={schedule('now')}
                disabled={disabled}
                className="rounded-[8px] z-[300] disabled:cursor-not-allowed disabled:opacity-80 hidden group-hover:flex absolute bottom-[100%] -left-[12px] p-[12px] w-[206px] bg-newBgColorInner"
              >
                <div className="text-white rounded-[8px] bg-[#D82D7E] h-[44px] w-full flex justify-center items-center post-now">{t('post_now', 'Post Now')}</div>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
