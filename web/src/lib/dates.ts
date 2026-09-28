import dayjs from 'dayjs';
import 'dayjs/locale/en';
import 'dayjs/locale/es';
import isoWeek from 'dayjs/plugin/isoWeek';
import isSameOrAfter from 'dayjs/plugin/isSameOrAfter';
import isSameOrBefore from 'dayjs/plugin/isSameOrBefore';
import localizedFormat from 'dayjs/plugin/localizedFormat';
import utc from 'dayjs/plugin/utc';
import weekOfYear from 'dayjs/plugin/weekOfYear';
import i18next from 'i18next';

dayjs.extend(utc);
dayjs.extend(isoWeek);
dayjs.extend(weekOfYear);
dayjs.extend(isSameOrAfter);
dayjs.extend(isSameOrBefore);
dayjs.extend(localizedFormat);

export function syncDayjsLocale() {
  dayjs.locale(i18next.resolvedLanguage === 'es' ? 'es' : 'en');
}

// newDayjs is local time, like Postiz's helper of the same name.
export const newDayjs = (value?: dayjs.ConfigType) => dayjs(value);

export { dayjs };
