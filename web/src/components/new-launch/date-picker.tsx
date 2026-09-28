// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/helpers/date.picker.tsx (AGPL-3.0).
// El calendario de Mantine se sustituye por una rejilla propia con las mismas clases.
import { FC, useCallback, useMemo, useState } from 'react';
import clsx from 'clsx';
import { useT } from '@/lib/i18n';
import { dayjs, newDayjs } from '@/lib/dates';
import { Button } from '@/components/ui/button';
import { CalendarIcon, ChevronLeftIcon, ChevronRightIcon } from '@/components/ui/icons';
import { useClickOutside } from '@/components/launches/select-customer';

const isUSCitizen = () => (navigator.language || navigator.languages[0] || '').startsWith('en-US');

export const DatePicker: FC<{ date: dayjs.Dayjs; onChange: (day: dayjs.Dayjs) => void }> = ({ date, onChange }) => {
  const t = useT();
  const [open, setOpen] = useState(false);
  const changeShow = useCallback(() => setOpen((prev) => !prev), []);
  const close = useCallback(() => setOpen(false), []);
  const ref = useClickOutside<HTMLDivElement>(close);

  const changeDay = useCallback(
    (day: dayjs.Dayjs) => onChange(newDayjs(`${day.format('YYYY-MM-DD')} ${date.format('HH:mm:ss')}`)),
    [date, onChange]
  );
  const changeTime = useCallback(
    (time: string) => {
      if (/^\d{2}:\d{2}$/.test(time)) {
        onChange(newDayjs(`${date.format('YYYY-MM-DD')} ${time}:00`));
      }
    },
    [date, onChange]
  );

  return (
    <div
      className="px-[16px] border border-newTextColor/10 rounded-[8px] justify-center flex gap-[8px] items-center relative h-[44px] text-[15px] font-[600] ml-[7px] select-none flex-1"
      onClick={changeShow}
      ref={ref}
      data-testid="post-date"
    >
      <div className="cursor-pointer">
        <CalendarIcon />
      </div>
      <div className="cursor-pointer">{date.format(isUSCitizen() ? 'MM/DD/YYYY hh:mm A' : 'DD/MM/YYYY HH:mm')}</div>
      {open && (
        <div
          onClick={(e) => e.stopPropagation()}
          className="animate-fadeIn absolute bottom-[100%] mb-[16px] start-[50%] -translate-x-[50%] bg-sixth border border-tableBorder text-textColor rounded-[16px] z-[300] p-[16px] flex flex-col"
        >
          <MonthGrid value={date} onChange={changeDay} />
          <label className="text-textColor py-[12px] text-[14px] font-[500]" htmlFor="post-time">
            {t('pick_time', 'Pick time')}
          </label>
          <input
            id="post-time"
            type="time"
            className="bg-sixth h-[40px] border border-tableBorder text-textColor rounded-[4px] outline-none px-[12px] font-[400]"
            defaultValue={date.format('HH:mm')}
            onChange={(e) => changeTime(e.target.value)}
          />
          <Button className="mt-[12px]" onClick={changeShow}>
            {t('close', 'Close')}
          </Button>
        </div>
      )}
    </div>
  );
};

const MonthGrid: FC<{ value: dayjs.Dayjs; onChange: (day: dayjs.Dayjs) => void }> = ({ value, onChange }) => {
  const [month, setMonth] = useState(() => value.startOf('month'));
  const days = useMemo(() => {
    const first = month.startOf('isoWeek');
    const last = month.endOf('month').endOf('isoWeek');
    const list: dayjs.Dayjs[] = [];
    for (let day = first; !day.isAfter(last, 'day'); day = day.add(1, 'day')) {
      list.push(day);
    }
    return list;
  }, [month]);
  const weekdays = useMemo(() => days.slice(0, 7).map((d) => d.format('dd')), [days]);

  return (
    <div className="w-[260px] text-[14px] font-[400]">
      <div className="flex items-center justify-between mb-[8px]">
        <button type="button" className="w-[28px] h-[28px] rounded-[4px] flex justify-center items-center text-textColor hover:bg-third" onClick={() => setMonth(month.subtract(1, 'month'))}>
          <ChevronLeftIcon />
        </button>
        <div className="px-[8px] h-[28px] rounded-[4px] flex items-center text-textColor hover:bg-third capitalize">{month.format('MMMM YYYY')}</div>
        <button type="button" className="w-[28px] h-[28px] rounded-[4px] flex justify-center items-center text-textColor hover:bg-third" onClick={() => setMonth(month.add(1, 'month'))}>
          <ChevronRightIcon />
        </button>
      </div>
      <div className="grid grid-cols-7">
        {weekdays.map((weekday) => (
          <div key={weekday} className="h-[28px] flex justify-center items-center text-[12px] text-gray capitalize">
            {weekday}
          </div>
        ))}
        {days.map((day) => {
          const outside = day.month() !== month.month();
          const selected = day.isSame(value, 'day');
          const weekend = day.isoWeekday() >= 6;
          return (
            <button
              type="button"
              key={day.format('YYYY-MM-DD')}
              onClick={() => onChange(day)}
              data-testid={`pick-day-${day.format('YYYY-MM-DD')}`}
              className={clsx(
                'h-[36px] rounded-[4px] hover:bg-seventh',
                selected ? '!text-white !bg-seventh !outline-none' : outside ? '!text-gray' : weekend ? '!text-customColor28' : '!text-textColor'
              )}
            >
              {day.date()}
            </button>
          );
        })}
      </div>
    </div>
  );
};
