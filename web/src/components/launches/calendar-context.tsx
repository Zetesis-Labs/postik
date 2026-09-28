// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/calendar.context.tsx (AGPL-3.0).
// Los datos salen de nuestra API con TanStack Query en vez de SWR.
import { createContext, FC, ReactNode, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, CalendarPost, Channel } from '@/lib/api';
import { dayjs, newDayjs } from '@/lib/dates';

export type ListStateFilter = 'all' | 'scheduled' | 'draft' | 'published';
export type Display = 'day' | 'week' | 'month' | 'list';

// Integrations is the channel as the ported calendar and editor expect it.
export interface Integrations {
  name: string;
  id: string;
  disabled?: boolean;
  inBetweenSteps: boolean;
  refreshNeeded?: boolean;
  identifier: string;
  picture: string;
  display: string;
  time: { time: number }[];
  customer?: { name?: string; id?: string };
}

export function toIntegration(channel: Channel): Integrations {
  return {
    id: channel.id,
    name: channel.name,
    disabled: channel.disabled,
    inBetweenSteps: channel.inBetweenSteps,
    refreshNeeded: channel.refreshNeeded,
    identifier: channel.provider,
    picture: channel.picture ?? '',
    display: channel.username ? `@${channel.username}` : '',
    time: channel.postingTimes.map((time) => ({ time })),
    customer: channel.customer ? { id: channel.customer.id, name: channel.customer.name } : undefined,
  };
}

type Filters = { startDate: string; endDate: string; display: Display; customer: string | null };

const displayKey = 'postik_calendar_display';

function readDisplay(): Display {
  try {
    const saved = window.localStorage.getItem(displayKey);
    if (saved === 'day' || saved === 'week' || saved === 'month' || saved === 'list') {
      return saved;
    }
  } catch {
    // Without storage the week view is the default.
  }
  return 'week';
}

export function getDateRange(display: Display, referenceDate?: string) {
  const date = referenceDate ? newDayjs(referenceDate) : newDayjs();
  switch (display) {
    case 'day':
    case 'list':
      return { startDate: date.format('YYYY-MM-DD'), endDate: date.format('YYYY-MM-DD') };
    case 'month':
      return { startDate: date.startOf('month').format('YYYY-MM-DD'), endDate: date.endOf('month').format('YYYY-MM-DD') };
    default:
      return { startDate: date.startOf('isoWeek').format('YYYY-MM-DD'), endDate: date.endOf('isoWeek').format('YYYY-MM-DD') };
  }
}

type CalendarValue = Filters & {
  loading: boolean;
  integrations: Integrations[];
  posts: CalendarPost[];
  reloadCalendarView: () => void;
  setFilters: (filters: Filters) => void;
  changeDate: (id: string, date: dayjs.Dayjs) => void;
  listPosts: CalendarPost[];
  listPage: number;
  listTotalPages: number;
  setListPage: (page: number) => void;
  listState: ListStateFilter;
  setListState: (state: ListStateFilter) => void;
};

export const CalendarContext = createContext<CalendarValue>({
  ...getDateRange('week'),
  display: 'week',
  customer: null,
  loading: true,
  integrations: [],
  posts: [],
  reloadCalendarView: () => {},
  setFilters: () => {},
  changeDate: () => {},
  listPosts: [],
  listPage: 0,
  listTotalPages: 0,
  setListPage: () => {},
  listState: 'all',
  setListState: () => {},
});

export const CalendarWeekProvider: FC<{ children: ReactNode; integrations: Integrations[] }> = ({ children, integrations }) => {
  const queryClient = useQueryClient();
  const [filters, setFiltersState] = useState<Filters>(() => {
    const search = new URLSearchParams(window.location.search);
    const display = (search.get('display') as Display) || readDisplay();
    const start = search.get('startDate');
    const end = search.get('endDate');
    const range = start && end ? { startDate: start, endDate: end } : getDateRange(display);
    return { ...range, display, customer: search.get('customer') };
  });
  const [listPage, setListPage] = useState(0);
  const [listState, setListStateRaw] = useState<ListStateFilter>('all');
  const setListState = useCallback((next: ListStateFilter) => {
    setListStateRaw(next);
    setListPage(0);
  }, []);

  const from = newDayjs(filters.startDate).startOf('day').utc().format();
  const to = newDayjs(filters.endDate).endOf('day').add(1, 'millisecond').utc().format();
  const calendar = useQuery({
    queryKey: ['posts', 'range', from, to, filters.customer],
    enabled: filters.display !== 'list',
    placeholderData: keepPreviousData,
    queryFn: async () => {
      const { data } = await api.GET('/posts', {
        params: { query: { from, to, ...(filters.customer ? { customer: filters.customer } : {}) } },
      });
      return data ?? [];
    },
  });
  const list = useQuery({
    queryKey: ['posts', 'list', listPage, listState],
    enabled: filters.display === 'list',
    queryFn: async () => {
      const { data } = await api.GET('/posts/list', { params: { query: { page: listPage + 1, status: listState } } });
      return data;
    },
  });

  const [internalData, setInternalData] = useState<CalendarPost[]>([]);
  useEffect(() => {
    setInternalData(calendar.data ?? []);
  }, [calendar.data]);

  const setFilters = useCallback((next: Filters) => {
    try {
      window.localStorage.setItem(displayKey, next.display);
    } catch {
      // The view is not remembered without storage.
    }
    setFiltersState(next);
    if (next.display === 'list') {
      setListPage(0);
    }
    const path = [
      `startDate=${next.startDate}`,
      `endDate=${next.endDate}`,
      `display=${next.display}`,
      next.customer ? `customer=${next.customer}` : '',
    ].filter(Boolean);
    window.history.replaceState(null, '', `/launches?${path.join('&')}`);
  }, []);

  const changeDate = useCallback((id: string, date: dayjs.Dayjs) => {
    setInternalData((d) => d.map((post) => (post.id === id ? { ...post, publishAt: date.utc().format() } : post)));
  }, []);

  const reloadCalendarView = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ['posts'] });
  }, [queryClient]);

  const value = useMemo<CalendarValue>(
    () => ({
      ...filters,
      loading: filters.display === 'list' ? list.isLoading : calendar.isLoading,
      integrations,
      posts: internalData,
      reloadCalendarView,
      setFilters,
      changeDate,
      listPosts: list.data?.items ?? [],
      listPage,
      listTotalPages: list.data?.pages ?? 0,
      setListPage,
      listState,
      setListState,
    }),
    [filters, list.isLoading, list.data, calendar.isLoading, integrations, internalData, reloadCalendarView, setFilters, changeDate, listPage, listState, setListState]
  );

  return <CalendarContext.Provider value={value}>{children}</CalendarContext.Provider>;
};

export const useCalendar = () => useContext(CalendarContext);
