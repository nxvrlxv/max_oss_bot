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

/** Нагрузка кнопки или ссылки, с которой открыли приложение: «open_42», «new». */
export function startParam(): string {
  const fromBridge = app()?.initDataUnsafe?.start_param;
  if (fromBridge) return fromBridge;
  // Для разработки в браузере: http://localhost:5173/?start=open_1
  return new URLSearchParams(window.location.search).get('start') ?? '';
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
