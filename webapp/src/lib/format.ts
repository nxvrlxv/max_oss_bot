// Числа и даты по-русски. Время собраний — московское: так его пишут
// в уведомлениях и протоколе, где бы ни находился собственник.

const TIME_ZONE = 'Europe/Moscow';

const areaFormat = new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 1 });
const areaExact = new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 });

/** 2541.3 → «2 541,3 м²» */
export function area(value: number, exact = false): string {
  return `${(exact ? areaExact : areaFormat).format(value)} м²`;
}

const percentFormat = new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 1 });

/** 0.7 → «70%», 0.1167 → «11,7%». Доля от 0 до 1. */
export function percent(share: number): string {
  return `${percentFormat.format(share * 100)}%`;
}

const dayFormat = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long', timeZone: TIME_ZONE });
const dayTimeFormat = new Intl.DateTimeFormat('ru-RU', {
  day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit', timeZone: TIME_ZONE,
});

/** «30 сентября» */
export function day(iso?: string): string {
  return iso ? dayFormat.format(new Date(iso)) : '';
}

/** «30 сентября, 20:00» */
export function dayTime(iso?: string): string {
  return iso ? dayTimeFormat.format(new Date(iso)).replace(' в ', ', ') : '';
}

/** «Кузнецова Елена Викторовна» → «Кузнецова Е. В.»; организации не трогаем. */
export function shortName(name: string): string {
  const parts = name.trim().split(/\s+/);
  if (parts.length < 2 || /[«"]|ООО|АО|образование/i.test(name)) return name;
  return `${parts[0]} ${parts.slice(1).map((p) => `${p[0]}.`).join(' ')}`;
}

/** Инициалы для аватара: «Кузнецова Елена» → «КЕ» */
export function initials(name: string): string {
  const parts = name.replace(/[«»"]/g, '').trim().split(/\s+/).filter(Boolean);
  return parts.slice(0, 2).map((p) => p[0]?.toUpperCase() ?? '').join('') || '?';
}

/** Склонение: plural(3, ['квартира', 'квартиры', 'квартир']) */
export function plural(n: number, forms: [string, string, string]): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return forms[0];
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return forms[1];
  return forms[2];
}

/** datetime-local ↔ ISO в московском времени. */
export function toMoscowInput(iso: string): string {
  const date = new Date(new Date(iso).getTime() + 3 * 60 * 60 * 1000);
  return date.toISOString().slice(0, 16);
}

export function fromMoscowInput(value: string): string {
  return new Date(`${value}:00+03:00`).toISOString();
}
