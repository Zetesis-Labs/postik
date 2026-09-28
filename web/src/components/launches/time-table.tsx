// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/time.table.tsx (AGPL-3.0).
// Borra la franja por su valor: Postiz la borraba por su posición en la lista ordenada.
import { FC, useCallback, useMemo, useState } from 'react';
import clsx from 'clsx';
import { api, Channel } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { Button } from '@/components/ui/button';
import { DelayIcon, PlusIcon, TrashIcon } from '@/components/ui/icons';
import { deleteDialog, useModals } from '@/components/ui/modals';
import { Select } from '@/components/ui/select';

const hours = [...Array(24).keys()];
const minutes = [...Array(60).keys()];
const pad = (n: number) => n.toString().padStart(2, '0');

// The slots are minutes after midnight UTC; the screen shows them in local time.
export function localToUtcMinutes(hour: number, minute: number, offsetMinutes: number): number {
  return (((hour * 60 + minute + offsetMinutes) % 1440) + 1440) % 1440;
}

export function formatSlot(utcMinutes: number): string {
  const date = new Date(Date.UTC(2000, 0, 1, 0, utcMinutes));
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export const TimeTable: FC<{ integration: Channel; mutate: () => void }> = ({ integration, mutate }) => {
  const t = useT();
  const modal = useModals();
  const [currentTimes, setCurrentTimes] = useState<number[]>([...integration.postingTimes]);
  const [hour, setHour] = useState(0);
  const [minute, setMinute] = useState(0);

  const removeSlot = useCallback(
    (value: number) => async () => {
      if (!(await deleteDialog(t('are_you_sure_you_want_to_delete_this_slot', 'Are you sure you want to delete this slot?')))) {
        return;
      }
      setCurrentTimes((prev) => prev.filter((time) => time !== value));
    },
    [t]
  );

  const addHour = useCallback(() => {
    const offset = new Date().getTimezoneOffset();
    setCurrentTimes((prev) => [...prev, localToUtcMinutes(hour, minute, offset)]);
  }, [hour, minute]);

  const times = useMemo(
    () =>
      [...new Set(currentTimes)]
        .sort((a, b) => a - b)
        .map((value) => ({ value, formatted: formatSlot(value) })),
    [currentTimes]
  );

  const save = useCallback(async () => {
    await api.PUT('/channels/{id}/posting-times', {
      params: { path: { id: integration.id } },
      body: { times: currentTimes },
    });
    mutate();
    modal.closeAll();
  }, [currentTimes, integration.id, modal, mutate]);

  return (
    <div className="relative w-full max-w-[400px] mx-auto">
      <div className="bg-newBgColorInner rounded-[12px] p-[20px] border border-newTableBorder">
        <div className="text-[15px] font-semibold mb-[16px] flex items-center gap-[8px]">
          <DelayIcon size={18} className="text-[#612BD3]" />
          {t('add_time_slot', 'Add Time Slot')}
        </div>

        <div className="flex gap-[12px] items-end">
          <div className="flex-1">
            <Select
              label={t('hour', 'Hour')}
              name="hour"
              disableForm={true}
              hideErrors={true}
              value={hour}
              onChange={(e) => setHour(Number(e.target.value))}
            >
              {hours.map((h) => (
                <option key={h} value={h}>
                  {pad(h)}
                </option>
              ))}
            </Select>
          </div>
          <div className="flex-1">
            <Select
              label={t('minutes', 'Minutes')}
              name="minutes"
              disableForm={true}
              hideErrors={true}
              value={minute}
              onChange={(e) => setMinute(Number(e.target.value))}
            >
              {minutes.map((m) => (
                <option key={m} value={m}>
                  {pad(m)}
                </option>
              ))}
            </Select>
          </div>
          <button
            type="button"
            onClick={addHour}
            className="h-[42px] px-[16px] bg-[#612BD3] hover:bg-[#7640e0] transition-colors rounded-[8px] flex items-center gap-[6px] text-white text-[14px] font-medium"
          >
            <PlusIcon size={14} />
            {t('add', 'Add')}
          </button>
        </div>
      </div>

      <div className="mt-[20px]">
        <div className="text-[14px] text-newTextColor/60 mb-[12px]">
          {t('scheduled_times', 'Scheduled Times')} ({times.length})
        </div>

        {times.length === 0 ? (
          <div className="text-center py-[32px] text-newTextColor/40 text-[14px] border border-dashed border-newTableBorder rounded-[12px]">
            {t('no_time_slots', 'No time slots added yet')}
          </div>
        ) : (
          <div className="flex flex-col gap-[8px]" data-testid="time-slots">
            {times.map((timeSlot) => (
              <div
                key={timeSlot.value}
                className={clsx(
                  'group flex items-center justify-between',
                  'h-[48px] px-[16px] rounded-[8px]',
                  'bg-newBgColorInner border border-newTableBorder',
                  'hover:border-[#612BD3]/40 transition-colors'
                )}
              >
                <div className="flex items-center gap-[12px]">
                  <div className="w-[8px] h-[8px] rounded-full bg-[#612BD3]" />
                  <span className="text-[15px] font-medium tabular-nums">{timeSlot.formatted}</span>
                </div>
                <button
                  type="button"
                  onClick={removeSlot(timeSlot.value)}
                  className="opacity-0 group-hover:opacity-100 transition-opacity p-[8px] hover:bg-red-500/10 rounded-[6px] text-red-400 hover:text-red-500"
                >
                  <TrashIcon size={16} />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="mt-[24px]">
        <Button type="button" className="w-full rounded-[8px]" onClick={save}>
          {t('save_changes', 'Save Changes')}
        </Button>
      </div>
    </div>
  );
};
