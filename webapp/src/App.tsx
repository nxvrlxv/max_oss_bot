import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { Diagnostics } from './components/Diagnostics';
import { haptic, insideMax, setNativeBack, startParam } from './lib/bridge';
import { NavContext, routeFromStart, type Nav, type Route } from './lib/nav';
import { Home } from './screens/owner/Home';
import { Join } from './screens/owner/Join';
import { Open } from './screens/owner/Open';
import { PickFlat } from './screens/owner/PickFlat';
import { Vote } from './screens/owner/Vote';
import { Claims } from './screens/initiator/Claims';
import { Dashboard } from './screens/initiator/Dashboard';
import { MeetingForm } from './screens/initiator/MeetingForm';
import { Setup } from './screens/initiator/Setup';

// Журнал запуска для панели диагностики: что и когда приходило от MAX.
const launchEvents: string[] = [];
function logLaunch(event: string) {
  launchEvents.push(`${new Date().toLocaleTimeString('ru-RU')} ${event}`);
}

function initialStack(): Route[] {
  const param = startParam();
  const start = routeFromStart(param);
  logLaunch(`запуск: start_param=«${param}» → ${start.name}`);
  // Под любым экраном, открытым по ссылке, лежит главный: «назад» ведёт к списку.
  return start.name === 'home' ? [start] : [{ name: 'home' }, start];
}

export function App() {
  const [stack, setStack] = useState<Route[]>(initialStack);
  const [error, setError] = useState<string | null>(null);
  const [diagnostics, setDiagnostics] = useState(false);
  const errorTimer = useRef<number>(undefined);
  const handledStart = useRef(startParam());

  const route = stack[stack.length - 1];

  const back = useCallback(() => setStack((s) => (s.length > 1 ? s.slice(0, -1) : s)), []);

  const nav = useMemo<Nav>(() => ({
    go: (next) => setStack((s) => [...s, next]),
    replace: (next) => setStack((s) => [...s.slice(0, -1), next]),
    back,
    reset: (next) => setStack(next.name === 'home' ? [next] : [{ name: 'home' }, next]),
    canGoBack: stack.length > 1,
    showError: (message) => {
      haptic.error();
      setError(message);
      window.clearTimeout(errorTimer.current);
      errorTimer.current = window.setTimeout(() => setError(null), 4000);
    },
    showNotice: (message) => {
      setError(message);
      window.clearTimeout(errorTimer.current);
      errorTimer.current = window.setTimeout(() => setError(null), 2500);
    },
    openDiagnostics: () => setDiagnostics(true),
  }), [back, stack.length]);

  // Повторное открытие по другой ссылке, пока приложение уже запущено:
  // MAX может не перезагружать страницу, а только сменить адрес. Bridge
  // этого не замечает, поэтому следим сами и переходим на новое собрание.
  useEffect(() => {
    const check = (reason: string) => {
      const param = startParam();
      logLaunch(`${reason}: start_param=«${param}»`);
      if (param && param !== handledStart.current) {
        handledStart.current = param;
        nav.reset(routeFromStart(param));
      }
    };
    const onHash = () => check('смена адреса');
    const onVisible = () => { if (document.visibilityState === 'visible') check('возврат в приложение'); };
    window.addEventListener('hashchange', onHash);
    document.addEventListener('visibilitychange', onVisible);
    return () => {
      window.removeEventListener('hashchange', onHash);
      document.removeEventListener('visibilitychange', onVisible);
    };
  }, [nav]);

  // Нативная кнопка «Назад» MAX — на всех экранах, кроме корневого.
  useEffect(() => setNativeBack(stack.length > 1 ? back : null), [stack.length, back]);

  // Фигурные скобки нужны: в свежем Chromium scrollTo возвращает Promise, а эффект должен вернуть функцию или ничего.
  useEffect(() => { window.scrollTo(0, 0); }, [route]);

  return (
    <NavContext.Provider value={nav}>
      {!insideMax() && stack.length > 1 && (
        <button type="button" className="back" onClick={back}>‹ Назад</button>
      )}
      <Screen route={route} />
      {error && <div className="error-toast" role="alert" onClick={() => setError(null)}>{error}</div>}
      {diagnostics && <Diagnostics events={launchEvents} onClose={() => setDiagnostics(false)} />}
    </NavContext.Provider>
  );
}

function Screen({ route }: { route: Route }) {
  // key — чтобы при переходе между собраниями состояние экрана не переезжало.
  switch (route.name) {
    case 'home': return <Home />;
    case 'join': return <Join key={route.token} token={route.token} />;
    case 'open': return <Open key={route.id} id={route.id} />;
    case 'pick': return <PickFlat key={route.id} id={route.id} />;
    case 'vote': return <Vote key={route.id} id={route.id} />;
    case 'create': return <MeetingForm />;
    case 'edit': return <MeetingForm key={route.id} id={route.id} />;
    case 'setup': return <Setup key={route.id} id={route.id} />;
    case 'dashboard': return <Dashboard key={route.id} id={route.id} />;
    case 'claims': return <Claims key={route.id} id={route.id} />;
  }
}
