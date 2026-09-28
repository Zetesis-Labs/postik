// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/calendar.tsx (AGPL-3.0).
// Sin estadísticas, vista previa pública, JSON de depuración, sets ni firmas (funcional §9).
import { FC, Fragment, memo, useCallback, useEffect, useMemo, useState } from 'react';
import clsx from 'clsx';
import { useDrag, useDrop } from 'react-dnd';
import { api, CalendarPost } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { dayjs, newDayjs, syncDayjsLocale } from '@/lib/dates';
import { deleteDialog, useModals } from '@/components/ui/modals';
import { useToaster } from '@/components/ui/toaster';
import { Button } from '@/components/ui/button';
import { useAddProvider } from '@/components/launches/add-provider';
import { CalendarContext, Integrations, useCalendar } from '@/components/launches/calendar-context';
import { openPostEditor } from '@/components/new-launch/open-editor';
import { MissingReleaseModal } from '@/components/launches/missing-release';

export const hours = Array.from({ length: 24 }, (_, i) => i);

const convertTimeFormatBasedOnLocality = (time: number) => `${time}:00`;

// Shared hook for post actions (edit, duplicate, delete).
const usePostActions = () => {
  const t = useT();
  const modal = useModals();
  const toaster = useToaster();
  const { integrations, reloadCalendarView } = useCalendar();

  const editPost = useCallback(
    (post: CalendarPost, isDuplicate?: boolean) => async () => {
      const { data } = await api.GET('/posts/{id}', { params: { path: { id: post.id } } });
      if (!data) {
        return;
      }
      let date = newDayjs(data.publishAt);
      if (isDuplicate) {
        const slot = await api.GET('/posts/next-slot');
        date = slot.data ? newDayjs(slot.data.date) : newDayjs().add(10, 'minute');
      }
      openPostEditor(modal, {
        integrations,
        mutate: reloadCalendarView,
        date,
        ...(isDuplicate
          ? { onlyValues: data.values.map((v) => ({ content: v.content, media: v.media })) }
          : { existing: data }),
      });
    },
    [integrations, modal, reloadCalendarView]
  );

  const deletePost = useCallback(
    (post: CalendarPost) => async () => {
      if (!(await deleteDialog(t('are_you_sure_you_want_to_delete_post', 'Are you sure you want to delete post?')))) {
        return;
      }
      await api.DELETE('/posts/{id}/group', { params: { path: { id: post.id } } });
      toaster.show(t('post_deleted_successfully', 'Post deleted successfully'), 'success');
      reloadCalendarView();
    },
    [reloadCalendarView, t, toaster]
  );

  const openMissingRelease = useCallback(
    (post: CalendarPost) => () => {
      modal.openModal({
        title: t('link_publication', 'Link publication'),
        withCloseButton: true,
        children: <MissingReleaseModal postId={post.id} onSuccess={reloadCalendarView} />,
      });
    },
    [modal, reloadCalendarView, t]
  );

  return { editPost, deletePost, openMissingRelease };
};

const localTime = (post: CalendarPost) => dayjs.utc(post.publishAt).local();

export const DayView = () => {
  const calendar = useCalendar();
  const { integrations, posts, startDate } = calendar;
  syncDayjsLocale();
  const currentDay = newDayjs(startDate);

  const options = useMemo(() => {
    type Option = { integration: Integrations[]; time: number };
    const byTime = new Map<number, Option[]>();
    const push = (option: Option) => byTime.set(option.time, [...(byTime.get(option.time) ?? []), option]);
    for (const post of posts) {
      const integration = integrations.find((i) => i.id === post.channel.id);
      const at = localTime(post);
      push({ integration: integration ? [integration] : [], time: at.diff(at.startOf('day'), 'minute') });
    }
    for (const integration of integrations) {
      for (const slot of integration.time) {
        const local = dayjs.utc().startOf('day').add(slot.time, 'minute').local();
        push({ integration: [integration], time: local.diff(local.startOf('day'), 'minute') });
      }
    }
    return [...byTime.entries()].sort(([a], [b]) => a - b).map(([, list]) => list);
  }, [integrations, posts]);

  return (
    <div className="flex flex-col gap-[10px] flex-1 relative">
      <div className="absolute start-0 top-0 w-full h-full flex flex-col overflow-auto scrollbar scrollbar-thumb-fifth scrollbar-track-newBgColor">
        {options.map((option) => (
          <Fragment key={option[0].time}>
            <div className="text-center text-[14px] min-h-[21px]">
              {currentDay.startOf('day').add(option[0].time, 'minute').format('LT')}
            </div>
            <div className="min-h-[60px] rounded-[10px] flex justify-center items-center gap-[10px] mb-[20px]">
              <CalendarContext.Provider
                value={{ ...calendar, integrations: [...new Set(option.flatMap((p) => p.integration))] }}
              >
                <CalendarColumn getDate={currentDay.startOf('day').add(option[0].time, 'minute')} />
              </CalendarContext.Provider>
            </div>
          </Fragment>
        ))}
      </div>
    </div>
  );
};

export const WeekView = () => {
  const { startDate } = useCalendar();
  syncDayjsLocale();
  const localizedDays = useMemo(() => {
    const weekStart = newDayjs(startDate);
    return Array.from({ length: 7 }, (_, i) => {
      const day = weekStart.add(i, 'day');
      return { name: day.format('dddd'), day: day.format('L'), date: day };
    });
  }, [startDate]);

  return (
    <div className="flex flex-col text-textColor flex-1">
      <div className="flex-1 relative">
        <div className="grid [grid-template-columns:136px_repeat(7,_minmax(0,_1fr))] gap-[4px] rounded-[10px] absolute h-full start-0 top-0 w-full overflow-auto scrollbar scrollbar-thumb-fifth scrollbar-track-newBgColor">
          <div className="z-10 bg-newTableHeader flex justify-center items-center flex-col h-[62px] rounded-[8px] sticky top-0"></div>
          {localizedDays.map((day) => (
            <div
              key={day.name}
              className="p-2 text-center bg-newTableHeader flex justify-center items-center flex-col h-[62px] rounded-[8px] sticky top-0 z-[20]"
            >
              <div className="text-[14px] font-[500] text-newTableText">{day.name}</div>
              <div
                className={clsx(
                  'text-[14px] font-[600] flex items-center justify-center gap-[6px]',
                  day.day === newDayjs().format('L') && 'text-newTableTextFocused'
                )}
              >
                {day.day === newDayjs().format('L') && <div className="w-[6px] h-[6px] bg-newTableTextFocused rounded-full" />}
                {day.day}
              </div>
            </div>
          ))}
          {hours.map((hour) => (
            <Fragment key={hour}>
              <div className="p-2 pe-4 text-center items-center justify-center flex text-[14px] text-newTableText">
                {convertTimeFormatBasedOnLocality(hour)}
              </div>
              {localizedDays.map((day) => (
                <Fragment key={`${startDate}-${day.date.format('YYYY-MM-DD')}-${hour}`}>
                  <div className="relative" data-testid={`slot-${day.date.format('YYYY-MM-DD')}-${hour}`}>
                    <CalendarColumn getDate={day.date.hour(hour).startOf('hour')} />
                  </div>
                </Fragment>
              ))}
            </Fragment>
          ))}
        </div>
      </div>
    </div>
  );
};

export const MonthView = () => {
  const { startDate } = useCalendar();
  syncDayjsLocale();
  const localizedDays = useMemo(() => Array.from({ length: 7 }, (_, i) => newDayjs().isoWeekday(i + 1).format('dddd')), []);

  const calendarDays = useMemo(() => {
    const monthStart = newDayjs(startDate).startOf('month');
    const calendarStartDate = monthStart.subtract(monthStart.isoWeekday() - 1, 'day');
    return Array.from({ length: 42 }, (_, i) => calendarStartDate.add(i, 'day'));
  }, [startDate]);

  return (
    <div className="flex flex-col text-textColor flex-1">
      <div className="flex-1 flex relative">
        <div className="grid grid-cols-7 grid-rows-[62px_auto] gap-[4px] rounded-[10px] absolute start-0 top-0 overflow-auto w-full h-full scrollbar scrollbar-thumb-tableBorder scrollbar-track-secondary">
          {localizedDays.map((day) => (
            <div key={day} className="z-[20] p-2 bg-newTableHeader flex justify-center items-center flex-col h-[62px] rounded-[8px] sticky top-0">
              <div>{day}</div>
            </div>
          ))}
          {calendarDays.map((date, index) => (
            <div key={index} className="text-center items-center justify-center flex">
              <CalendarColumn getDate={date.endOf('day')} randomHour={true} />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

export const ListView = () => {
  const t = useT();
  const { integrations, loading, listPosts, listState } = useCalendar();
  const emptyMessage =
    listState === 'scheduled'
      ? t('no_upcoming_posts', 'No upcoming posts scheduled')
      : listState === 'draft'
        ? t('no_draft_posts', 'No draft posts')
        : listState === 'published'
          ? t('no_published_posts', 'No published posts')
          : t('no_posts', 'No posts');
  const { editPost, deletePost, openMissingRelease } = usePostActions();

  const groupedPosts = useMemo(() => {
    const groups: Record<string, CalendarPost[]> = {};
    for (const post of listPosts) {
      const key = localTime(post).format('YYYY-MM-DD');
      (groups[key] ??= []).push(post);
    }
    return Object.entries(groups).sort(([a], [b]) => a.localeCompare(b));
  }, [listPosts]);

  if (loading) {
    return (
      <div className="flex flex-col flex-1 items-center justify-center">
        <div className="text-textColor">{t('loading', 'Loading...')}</div>
      </div>
    );
  }
  if (listPosts.length === 0) {
    return (
      <div className="flex flex-col flex-1 items-center justify-center">
        <div className="text-textColor text-[16px]">{emptyMessage}</div>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-[10px] flex-1 relative">
      <div className="absolute start-0 top-0 w-full h-full flex flex-col overflow-auto scrollbar scrollbar-thumb-fifth scrollbar-track-newBgColor">
        {groupedPosts.map(([dateKey, datePosts]) => (
          <Fragment key={dateKey}>
            <div className="text-center text-[14px] min-h-[21px] text-textColor font-[500] mt-[10px]">
              {newDayjs(dateKey).format('dddd, D MMMM YYYY')}
            </div>
            <div className="flex flex-col gap-[10px] mb-[20px] px-[10px]">
              {datePosts.map((post) => (
                <CalendarItem
                  key={post.id}
                  display="day"
                  isBeforeNow={false}
                  date={localTime(post)}
                  post={post}
                  integrations={integrations}
                  editPost={editPost(post, false)}
                  duplicatePost={editPost(post, true)}
                  deletePost={deletePost(post)}
                  missingRelease={openMissingRelease(post)}
                  showTime={true}
                />
              ))}
            </div>
          </Fragment>
        ))}
      </div>
    </div>
  );
};

export const Calendar = () => {
  const { display } = useCalendar();
  if (display === 'list') return <ListView />;
  if (display === 'day') return <DayView />;
  if (display === 'week') return <WeekView />;
  return <MonthView />;
};

export const CalendarColumn: FC<{ getDate: dayjs.Dayjs; randomHour?: boolean }> = memo(({ getDate, randomHour }) => {
  const t = useT();
  const [num, setNum] = useState(0);
  const { integrations, posts, changeDate, display, reloadCalendarView, loading } = useCalendar();
  const modal = useModals();
  const { editPost, deletePost, openMissingRelease } = usePostActions();

  const postList = useMemo(
    () =>
      posts.filter((post) => {
        const at = localTime(post);
        return display === 'day'
          ? at.format('YYYY-MM-DD HH:mm') === getDate.format('YYYY-MM-DD HH:mm')
          : display === 'week'
            ? at.isSameOrAfter(getDate.startOf('hour')) && at.isBefore(getDate.endOf('hour'))
            : at.format('DD/MM/YYYY') === getDate.format('DD/MM/YYYY');
      }),
    [posts, display, getDate]
  );
  const [showAll, setShowAll] = useState(false);
  const list = useMemo(() => (showAll ? postList : postList.slice(0, 3)), [postList, showAll]);

  const isBeforeNow = useMemo(() => getDate.startOf('hour').isBefore(newDayjs().startOf('hour')), [getDate, num]);

  useEffect(() => {
    if (isBeforeNow) {
      return;
    }
    const timer = setInterval(() => setNum((n) => n + 1), 120_000 + Math.random() * 30_000);
    return () => clearInterval(timer);
  }, [isBeforeNow]);

  const [{ canDrop }, drop] = useDrop(
    () => ({
      accept: 'post',
      drop: async (item: { id: string }) => {
        if (isBeforeNow) return;
        const post = posts.find((p) => p.id === item.id);
        let action: 'schedule' | 'update' = 'schedule';
        if (post && (post.status === 'published' || (post.status === 'scheduled' && newDayjs().isAfter(localTime(post))))) {
          const whatToDo = await new Promise<'schedule' | 'update' | 'cancel'>((resolve) => {
            modal.openModal({
              title: t('what_do_you_want_to_do', 'What do you want to do?'),
              onClose: () => resolve('cancel'),
              children: (
                <div className="flex flex-col">
                  <div className="text-[20px] mb-[20px]">
                    {t('post_already_published_republish_warning', 'This post was already published. Republishing will publish it again to')}{' '}
                    {post.channel.name} {t('republish_at', 'at')} {getDate.format('DD/MM/YYYY HH:mm')}.
                  </div>
                  <div className="flex w-full gap-[10px]">
                    <div className="flex-1 flex">
                      <Button type="button" className="flex-1" onClick={() => { modal.closeAll(); resolve('update'); }}>
                        {t('just_update_post_details', 'Just update the post details')}
                      </Button>
                    </div>
                    <div className="flex-1 flex">
                      <Button type="button" className="flex-1" onClick={() => { modal.closeAll(); resolve('schedule'); }}>
                        {t('reschedule_post', 'Reschedule the post')}
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
          action = whatToDo;
        }
        changeDate(item.id, getDate);
        await api.PUT('/posts/{id}/date', {
          params: { path: { id: item.id } },
          body: { publishAt: getDate.utc().format(), mode: action, ...(action === 'schedule' ? { republish: true } : {}) },
        });
        reloadCalendarView();
      },
      collect: (monitor) => ({ canDrop: isBeforeNow ? false : !!monitor.canDrop() && !!monitor.isOver() }),
    }),
    [posts, isBeforeNow, getDate]
  );

  const addModal = useCallback(() => {
    openPostEditor(modal, {
      integrations,
      mutate: reloadCalendarView,
      date: randomHour
        ? getDate.hour(Math.floor(Math.random() * 24)).startOf('hour')
        : getDate.format('YYYY-MM-DDTHH:mm') === newDayjs().startOf('hour').format('YYYY-MM-DDTHH:mm')
          ? newDayjs().add(10, 'minute')
          : getDate,
    });
  }, [integrations, getDate, modal, randomHour, reloadCalendarView]);

  const addProvider = useAddProvider();
  return (
    <div
      className={clsx(
        'flex flex-col w-full min-h-full relative',
        isBeforeNow && 'repeated-strip',
        loading && 'animate-pulse',
        isBeforeNow ? 'cursor-not-allowed' : 'border border-newTextColor/5 rounded-[8px]'
      )}
      ref={(node) => void drop(node)}
    >
      {display === 'month' && <div className={clsx('pt-[6px] text-[14px]')}>{getDate.date()}</div>}
      <div className={clsx('relative flex flex-col flex-1 text-white rounded-[8px] min-h-[70px]', canDrop && 'border border-[#612BD3]')}>
        <div
          className={clsx(
            'flex-col text-[12px] pointer w-full flex scrollbar scrollbar-thumb-tableBorder scrollbar-track-secondary',
            isBeforeNow ? 'flex-1' : 'cursor-pointer',
            isBeforeNow && postList.length === 0 && 'col-calendar'
          )}
        >
          {loading && (
            <div className="h-full w-full p-[5px] animate-pulse absolute left-0 top-0 z-[50]">
              <div className="h-full w-full bg-newSettings rounded-[10px]" />
            </div>
          )}
          {list.map((post) => (
            <div key={post.id} className={clsx('text-textColor p-[2.5px] relative flex flex-col justify-center items-center')}>
              <div className="relative w-full flex flex-col items-center p-[2.5px]">
                <CalendarItem
                  display={display as 'day' | 'week' | 'month'}
                  isBeforeNow={isBeforeNow}
                  date={getDate}
                  post={post}
                  integrations={integrations}
                  editPost={editPost(post, false)}
                  duplicatePost={editPost(post, true)}
                  deletePost={deletePost(post)}
                  missingRelease={openMissingRelease(post)}
                />
              </div>
            </div>
          ))}
          {!showAll && postList.length > 3 && (
            <div className="text-center hover:underline py-[5px] text-textColor" onClick={() => setShowAll(true)}>
              {t('show_more', '+ Show more')} ({postList.length - 3})
            </div>
          )}
          {showAll && postList.length > 3 && (
            <div className="text-center hover:underline py-[5px]" onClick={() => setShowAll(false)}>
              {t('show_less', '- Show less')}
            </div>
          )}
        </div>
        {!isBeforeNow && (
          <div className="pb-[2.5px] px-[5px] flex-1 flex" onClick={integrations.length ? addModal : addProvider} data-testid="add-post-slot">
            <div
              className={clsx(
                display === 'month' ? 'flex-1 min-h-[40px] w-full' : !postList.length ? 'min-h-full w-full p-[5px]' : 'min-h-[40px] w-full',
                'flex items-center justify-center cursor-pointer pb-[2.5px]'
              )}
            >
              {display !== 'day' && (
                <div className={clsx('group hover:before:h-[30px] w-full h-full rounded-[10px] flex justify-center items-center text-white')}>
                  <div
                    className={`group-hover:before:content-["+"] pb-[5px] flex justify-center items-center rounded-[8px] transition-all group-hover:bg-btnPrimary w-full h-full max-w-[40px] max-h-[40px]`}
                  />
                </div>
              )}
              {display === 'day' && (
                <div className="w-full h-full rounded-[10px] py-[10px] flex-wrap hover:border hover:border-seventh flex justify-center items-center gap-[20px] opacity-30 grayscale hover:grayscale-0 hover:opacity-100">
                  {integrations.map((selectedIntegrations) => (
                    <div className="relative" key={selectedIntegrations.id}>
                      <div className="relative w-[34px] h-[34px] rounded-[8px] flex justify-center items-center filter transition-all duration-500">
                        <img src={selectedIntegrations.picture || '/no-picture.jpg'} className="rounded-[8px]" alt={selectedIntegrations.identifier} width={32} height={32} />
                        <img
                          src={`/icons/platforms/${selectedIntegrations.identifier}.png`}
                          className="rounded-[8px] absolute z-10 -bottom-[5px] -end-[5px] border border-fifth"
                          alt={selectedIntegrations.identifier}
                          width={20}
                          height={20}
                        />
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
});

const CalendarItem: FC<{
  date: dayjs.Dayjs;
  isBeforeNow: boolean;
  editPost: () => void;
  duplicatePost: () => void;
  deletePost: () => void;
  missingRelease: () => void;
  integrations: Integrations[];
  display: 'day' | 'week' | 'month';
  showTime?: boolean;
  post: CalendarPost;
}> = memo(({ editPost, duplicatePost, post, isBeforeNow, deletePost, missingRelease, showTime }) => {
  const t = useT();
  const state = post.status;
  const [{ opacity }, dragRef] = useDrag(
    () => ({ type: 'post', item: { id: post.id }, collect: (monitor) => ({ opacity: monitor.isDragging() ? 0 : 1 }) }),
    [post.id]
  );
  const tagColor = post.tags[0]?.color;
  return (
    <div
      ref={(node) => void dragRef(node)}
      data-testid="calendar-post"
      className={clsx('w-full flex h-full flex-1 flex-col group', 'relative', state === 'error' && 'rounded-[10px] ring-2 ring-red-500')}
      style={{ opacity }}
    >
      {state === 'error' && (
        <div
          className="absolute -top-[6px] -left-[6px] z-20 w-[18px] h-[18px] rounded-full bg-red-500 flex items-center justify-center text-white text-[11px] font-bold cursor-pointer"
          data-tooltip-id="tooltip"
          data-tooltip-content={post.error ? t(`post_error_${post.error}`, post.error) : t('post_error_unknown', 'An error occurred while publishing this post')}
          data-testid="post-error"
        >
          !
        </div>
      )}
      <div
        className="text-white text-[11px] max-h-[24px] h-[24px] min-h-[24px] w-full rounded-tr-[10px] rounded-tl-[10px] flex items-center justify-center gap-[10px] px-[5px] bg-btnPrimary"
        style={{ backgroundColor: tagColor }}
      >
        <div className={clsx(tagColor ? 'mix-blend-difference' : '', 'group-hover:hidden cursor-pointer')}>
          {post.tags.map((p) => p.name).join(', ')}
        </div>
        <div className={clsx('hidden group-hover:block hover:underline cursor-pointer', tagColor && 'mix-blend-difference')} onClick={duplicatePost}>
          <Duplicate />
        </div>
        {state === 'published' && post.releaseUrl && (
          <a
            href={post.releaseUrl}
            target="_blank"
            rel="noreferrer"
            className={clsx('hidden group-hover:block hover:underline cursor-pointer', tagColor && 'mix-blend-difference')}
            data-testid="preview-post"
          >
            <Preview />
          </a>
        )}
        {state === 'published' && !post.releaseUrl && (
          <div
            className={clsx('hidden group-hover:block hover:underline cursor-pointer', tagColor && 'mix-blend-difference')}
            onClick={missingRelease}
            data-testid="missing-release"
          >
            <Statistics />
          </div>
        )}
        <div
          className={clsx('hidden group-hover:block hover:underline cursor-pointer', tagColor && 'mix-blend-difference')}
          onClick={deletePost}
          data-testid="delete-post"
        >
          <DeletePost />
        </div>
      </div>
      <div
        onClick={editPost}
        className={clsx(
          'gap-[5px] w-full flex h-full flex-1 rounded-br-[10px] rounded-bl-[10px] p-[8px] text-[14px] bg-newColColor',
          'relative',
          isBeforeNow && '!grayscale'
        )}
      >
        <div className={clsx('relative min-w-[20px]')}>
          <img className="w-[20px] h-[20px] rounded-[8px]" src={post.channel.picture || '/no-picture.jpg'} alt="" />
          <img
            className="w-[12px] h-[12px] rounded-[8px] absolute z-10 top-[10px] end-0 border border-fifth"
            src={`/icons/platforms/${post.channel.provider}.png`}
            alt=""
          />
        </div>
        <div className="w-full flex-1 flex flex-col min-h-[40px]">
          <div className="text-start">{state === 'draft' ? t('draft', 'Draft') + ': ' : ''}</div>
          <div className="w-full relative">
            <div className="absolute top-0 start-0 w-full text-ellipsis break-words line-clamp-1 text-start">
              {post.excerpt || t('no_content', 'no content')}
            </div>
          </div>
        </div>
        {showTime && (
          <div className="text-textColor/50 text-[12px] whitespace-nowrap flex items-center">{localTime(post).format('HH:mm')}</div>
        )}
      </div>
    </div>
  );
});

const Duplicate = () => {
  const t = useT();
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="15"
      height="15"
      viewBox="0 0 32 32"
      fill="none"
      data-tooltip-id="tooltip"
      data-tooltip-content={t('duplicate_post', 'Duplicate Post')}
    >
      <path
        d="M27 5H9C8.46957 5 7.96086 5.21071 7.58579 5.58579C7.21071 5.96086 7 6.46957 7 7V9H5C4.46957 9 3.96086 9.21071 3.58579 9.58579C3.21071 9.96086 3 10.4696 3 11V25C3 25.5304 3.21071 26.0391 3.58579 26.4142C3.96086 26.7893 4.46957 27 5 27H23C23.5304 27 24.0391 26.7893 24.4142 26.4142C24.7893 26.0391 25 25.5304 25 25V23H27C27.5304 23 28.0391 22.7893 28.4142 22.4142C28.7893 22.0391 29 21.5304 29 21V7C29 6.46957 28.7893 5.96086 28.4142 5.58579C28.0391 5.21071 27.5304 5 27 5ZM23 11V13H5V11H23ZM23 25H5V15H23V25ZM27 21H25V11C25 10.4696 24.7893 9.96086 24.4142 9.58579C24.0391 9.21071 23.5304 9 23 9H9V7H27V21Z"
        fill="currentColor"
      />
    </svg>
  );
};

const Preview = () => {
  const t = useT();
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="15"
      height="15"
      viewBox="0 0 32 32"
      fill="none"
      data-tooltip-id="tooltip"
      data-tooltip-content={t('preview_post', 'Preview Post')}
    >
      <path
        d="M30.9137 15.595C30.87 15.4963 29.8112 13.1475 27.4575 10.7937C24.3212 7.6575 20.36 6 16 6C11.64 6 7.67874 7.6575 4.54249 10.7937C2.18874 13.1475 1.12499 15.5 1.08624 15.595C1.02938 15.7229 1 15.8613 1 16.0012C1 16.1412 1.02938 16.2796 1.08624 16.4075C1.12999 16.5062 2.18874 18.8538 4.54249 21.2075C7.67874 24.3425 11.64 26 16 26C20.36 26 24.3212 24.3425 27.4575 21.2075C29.8112 18.8538 30.87 16.5062 30.9137 16.4075C30.9706 16.2796 31 16.1412 31 16.0012C31 15.8613 30.9706 15.7229 30.9137 15.595ZM16 24C12.1525 24 8.79124 22.6012 6.00874 19.8438C4.86704 18.7084 3.89572 17.4137 3.12499 16C3.89551 14.5862 4.86686 13.2915 6.00874 12.1562C8.79124 9.39875 12.1525 8 16 8C19.8475 8 23.2087 9.39875 25.9912 12.1562C27.1352 13.2912 28.1086 14.5859 28.8812 16C27.98 17.6825 24.0537 24 16 24ZM16 10C14.8133 10 13.6533 10.3519 12.6666 11.0112C11.6799 11.6705 10.9108 12.6075 10.4567 13.7039C10.0026 14.8003 9.88377 16.0067 10.1153 17.1705C10.3468 18.3344 10.9182 19.4035 11.7573 20.2426C12.5965 21.0818 13.6656 21.6532 14.8294 21.8847C15.9933 22.1162 17.1997 21.9974 18.2961 21.5433C19.3924 21.0892 20.3295 20.3201 20.9888 19.3334C21.6481 18.3467 22 17.1867 22 16C21.9983 14.4092 21.3657 12.884 20.2408 11.7592C19.1159 10.6343 17.5908 10.0017 16 10ZM16 20C15.2089 20 14.4355 19.7654 13.7777 19.3259C13.1199 18.8864 12.6072 18.2616 12.3045 17.5307C12.0017 16.7998 11.9225 15.9956 12.0768 15.2196C12.2312 14.4437 12.6122 13.731 13.1716 13.1716C13.731 12.6122 14.4437 12.2312 15.2196 12.0769C15.9956 11.9225 16.7998 12.0017 17.5307 12.3045C18.2616 12.6072 18.8863 13.1199 19.3259 13.7777C19.7654 14.4355 20 15.2089 20 16C20 17.0609 19.5786 18.0783 18.8284 18.8284C18.0783 19.5786 17.0609 20 16 20Z"
        fill="currentColor"
      />
    </svg>
  );
};

const Statistics = () => {
  const t = useT();
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="15"
      height="15"
      viewBox="0 0 32 32"
      fill="none"
      data-tooltip-id="tooltip"
      data-tooltip-content={t('link_publication', 'Link publication')}
    >
      <path
        d="M28 25H27V5C27 4.73478 26.8946 4.48043 26.7071 4.29289C26.5196 4.10536 26.2652 4 26 4H19C18.7348 4 18.4804 4.10536 18.2929 4.29289C18.1054 4.48043 18 4.73478 18 5V10H12C11.7348 10 11.4804 10.1054 11.2929 10.2929C11.1054 10.4804 11 10.7348 11 11V16H6C5.73478 16 5.48043 16.1054 5.29289 16.2929C5.10536 16.4804 5 16.7348 5 17V25H4C3.73478 25 3.48043 25.1054 3.29289 25.2929C3.10536 25.4804 3 25.7348 3 26C3 26.2652 3.10536 26.5196 3.29289 26.7071C3.48043 26.8946 3.73478 27 4 27H28C28.2652 27 28.5196 26.8946 28.7071 26.7071C28.8946 26.5196 29 26.2652 29 26C29 25.7348 28.8946 25.4804 28.7071 25.2929C28.5196 25.1054 28.2652 25 28 25ZM20 6H25V25H20V6ZM13 12H18V25H13V12ZM7 18H11V25H7V18Z"
        fill="currentColor"
      />
    </svg>
  );
};

export const DeletePost = () => {
  const t = useT();
  return (
    <svg
      width="15"
      height="15"
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      data-tooltip-id="tooltip"
      data-tooltip-content={t('delete_post', 'Delete Post')}
    >
      <path d="M15 10V18H9V10H15ZM14 4H9.9L8.9 5H6V7H18V5H15L14 4ZM17 8H7V18C7 19.1 7.9 20 9 20H15C16.1 20 17 19.1 17 18V8Z" fill="currentColor" />
    </svg>
  );
};
