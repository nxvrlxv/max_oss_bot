import { useEffect } from 'react';

import { api } from '../../api/client';
import { Failure, Loading, useLoad } from '../../components/ui';
import { useNav } from '../../lib/nav';

// Вход по ссылке «open_42»: сам решает, какой экран показать.
// Инициатору — дашборд или настройку черновика, собственнику — голосование
// или выбор квартиры, если заявки ещё нет.
export function Open({ id }: { id: number }) {
  const nav = useNav();
  const { data, error, reload } = useLoad(() => api.meeting(id), [id]);

  useEffect(() => {
    if (!data) return;
    if (data.is_initiator) {
      nav.replace(data.status === 'draft' ? { name: 'setup', id } : { name: 'dashboard', id });
    } else if (data.claims.length > 0 || data.status === 'finished') {
      nav.replace({ name: 'vote', id });
    } else {
      nav.replace({ name: 'pick', id });
    }
  }, [data, id, nav]);

  if (error) return <Failure message={error} onRetry={reload} />;
  return <Loading />;
}
