// Обёртка над MAX Bridge (window.WebApp). Вне MAX — в обычном браузере
// при разработке — объекта нет, и все вызовы становятся пустыми.

type ImpactStyle = 'soft' | 'light' | 'medium' | 'heavy' | 'rigid';
type NotificationType = 'error' | 'success' | 'warning';

interface MaxWebApp {
  initData?: string;
  initDataUnsafe?: {
    start_param?: string;
    user?: { id: number; first_name?: string; last_name?: string };
  };
  platform?: string;
  version?: string;
  getLaunchContext?(): Promise<unknown>;
  BackButton?: {
    show(): void;
    hide(): void;
    onClick(callback: () => void): void;
    offClick(callback: () => void): void;
  };
  HapticFeedback?: {
    impactOccurred(style: ImpactStyle): void;
    notificationOccurred(type: NotificationType): void;
    selectionChanged(): void;
  };
  openMaxLink?(url: string): void;
  openLink?(url: string): void;
  shareMaxContent?(params: { text?: string; link?: string }): Promise<unknown>;
  shareContent?(params: { text?: string; link?: string }): Promise<unknown>;
}

declare global {
  interface Window {
    WebApp?: MaxWebApp;
  }
}

function app(): MaxWebApp | undefined {
  return window.WebApp;
}

/** Строка для проверки подписи на сервере. Пустая — открыто не из MAX. */
export function initData(): string {
  return app()?.initData ?? '';
}

/**
 * Нагрузка кнопки или ссылки, с которой открыли приложение: «join_<токен>», «open_42».
 *
 * Сначала читаем адрес страницы: MAX кладёт туда #WebAppData=…&…, и при
 * повторном открытии уже запущенного приложения адрес меняется, а Bridge
 * свои данные не перечитывает — он разбирает их один раз при загрузке.
 */
export function startParam(): string {
  return hashStartParam() ?? app()?.initDataUnsafe?.start_param
    // Для разработки в браузере: http://localhost:5173/?start=open_1
    ?? new URLSearchParams(window.location.search).get('start') ?? '';
}

/** start_param из #WebAppData в адресе — так же, как его ищет Bridge. */
export function hashStartParam(): string | null {
  try {
    const data = new URLSearchParams(window.location.hash.replace(/^#/, '')).get('WebAppData');
    if (!data) return null;
    return new URLSearchParams(data).get('start_param')
      ?? new URLSearchParams(decodeURIComponent(data)).get('start_param');
  } catch {
    return null;
  }
}

/** Сырые данные для панели диагностики. Подпись initData сюда не попадает. */
export async function launchInfo(): Promise<Record<string, string>> {
  const webApp = app();
  const hashKeys = [...new URLSearchParams(window.location.hash.replace(/^#/, '')).keys()];
  const initKeys = webApp?.initData ? [...new URLSearchParams(webApp.initData).keys()] : [];

  let entryPoint = 'нет метода';
  if (webApp?.getLaunchContext) {
    entryPoint = await Promise.race([
      webApp.getLaunchContext().then((ctx) => JSON.stringify(ctx)).catch((err) => `ошибка: ${String(err)}`),
      new Promise<string>((resolve) => setTimeout(() => resolve('нет ответа за 2 с'), 2000)),
    ]);
  }

  return {
    'Bridge подключён': webApp ? 'да' : 'нет',
    'Открыто в MAX (есть initData)': insideMax() ? 'да' : 'нет',
    'Платформа': webApp?.platform ?? '—',
    'Версия MAX': webApp?.version ?? '—',
    'start_param от Bridge': webApp?.initDataUnsafe?.start_param ?? '—',
    'start_param из адреса': hashStartParam() ?? '—',
    'Итоговый start_param': startParam() || '—',
    'Пользователь': String(webApp?.initDataUnsafe?.user?.id ?? '—'),
    'Поля initData': initKeys.join(', ') || '—',
    'Поля в адресе (#)': hashKeys.join(', ') || '—',
    'Контекст запуска': entryPoint,
    'Страница загружена': new Date(performance.timeOrigin).toLocaleTimeString('ru-RU'),
    'Адрес без #': window.location.origin + window.location.pathname + window.location.search,
  };
}

/** Открыто внутри MAX — есть нативная кнопка «Назад». */
export function insideMax(): boolean {
  return Boolean(app()?.initData);
}

/** Показывает нативную кнопку «Назад» и вешает обработчик. Возвращает отписку. */
export function setNativeBack(onBack: (() => void) | null): () => void {
  const button = app()?.BackButton;
  if (!button) return () => {};
  if (!onBack) {
    button.hide();
    return () => {};
  }
  button.show();
  button.onClick(onBack);
  return () => { button.offClick(onBack); };
}

export const haptic = {
  success: () => app()?.HapticFeedback?.notificationOccurred('success'),
  error: () => app()?.HapticFeedback?.notificationOccurred('error'),
  select: () => app()?.HapticFeedback?.selectionChanged(),
  tap: () => app()?.HapticFeedback?.impactOccurred('light'),
};

export type ShareResult = 'shared' | 'copied' | 'failed';

/**
 * Делится ссылкой: сначала выбор чата внутри MAX, затем системное меню
 * «Поделиться», в обычном браузере — копирование в буфер обмена.
 */
export async function shareLink(link: string, text: string): Promise<ShareResult> {
  const webApp = app();
  for (const share of [webApp?.shareMaxContent, webApp?.shareContent]) {
    if (!share || !insideMax()) continue;
    try {
      await share.call(webApp, { text, link });
      return 'shared';
    } catch {
      // пользователь закрыл окно или метод не поддержан клиентом — пробуем следующий способ
    }
  }
  return copyText(link);
}

export async function copyText(value: string): Promise<ShareResult> {
  try {
    await navigator.clipboard.writeText(value);
    return 'copied';
  } catch {
    return 'failed';
  }
}
