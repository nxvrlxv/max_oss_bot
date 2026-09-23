import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { haptic, insideMax, setNativeBack, startParam } from './lib/bridge';
import { NavContext, routeFromStart, type Nav, type Route } from './lib/nav';
import { Home } from './screens/owner/Home';
import { Open } from './screens/owner/Open';
import { PickFlat } from './screens/owner/PickFlat';
import { Vote } from './screens/owner/Vote';
import { Claims } from './screens/initiator/Claims';
import { Dashboard } from './screens/initiator/Dashboard';
import { MeetingForm } from './screens/initiator/MeetingForm';
import { Setup } from './screens/initiator/Setup';

function initialStack(): Route[] {
  const start = routeFromStart(startParam());
  // Под любым экраном, открытым по ссылке, лежит главный: «назад» ведёт к списку.
  return start.name === 'home' ? [start] : [{ name: 'home' }, start];
}

export function App() {
  const [stack, setStack] = useState<Route[]>(initialStack);
  const [error, setError] = useState<string | null>(null);
  const errorTimer = useRef<number>(undefined);

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
  }), [back, stack.length]);

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
    </NavContext.Provider>
  );
}

function Screen({ route }: { route: Route }) {
  // key — чтобы при переходе между собраниями состояние экрана не переезжало.
  switch (route.name) {
    case 'home': return <Home />;
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
