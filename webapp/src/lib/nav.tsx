import { createContext, useContext } from 'react';

// Экраны приложения. Навигация — стек в памяти: мини-приложение живёт
// в одном окне MAX, адресная строка пользователю не видна.
export type Route =
  | { name: 'home' }
  | { name: 'open'; id: number } // решает, куда вести: инициатора — на дашборд, собственника — к голосованию
  | { name: 'pick'; id: number }
  | { name: 'vote'; id: number }
  | { name: 'create' }
  | { name: 'edit'; id: number }
  | { name: 'setup'; id: number }
  | { name: 'dashboard'; id: number }
  | { name: 'claims'; id: number };

export interface Nav {
  go(route: Route): void;
  replace(route: Route): void;
  back(): void;
  /** Сбрасывает стек: после публикации «назад» в форму не нужен. */
  reset(route: Route): void;
  canGoBack: boolean;
  showError(message: string): void;
}

export const NavContext = createContext<Nav | null>(null);

export function useNav(): Nav {
  const nav = useContext(NavContext);
  if (!nav) throw new Error('useNav вне NavContext');
  return nav;
}

/** Разбирает нагрузку запуска: «open_42», «claims_42», «new», «list». */
export function routeFromStart(param: string): Route {
  // «open_42»; двоеточие — старый формат кнопок.
  const [action, rawId] = param.split(/[_:]/);
  const id = Number(rawId);
  switch (action) {
    case 'open':
      return Number.isInteger(id) && id > 0 ? { name: 'open', id } : { name: 'home' };
    case 'claims':
      return Number.isInteger(id) && id > 0 ? { name: 'claims', id } : { name: 'home' };
    case 'new':
      return { name: 'create' };
    default:
      return { name: 'home' };
  }
}
