import { useEffect, useState } from 'react';

import { api, ApiError, rememberInvite } from '../../api/client';
import { Failure, Loading } from '../../components/ui';
import { useNav } from '../../lib/nav';

// Вход по приглашению: токен из ссылки превращается в номер собрания,
// а сам токен запоминается — с ним сервер покажет собрание постороннему.
export function Join({ token }: { token: string }) {
  const nav = useNav();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.join(token)
      .then(({ id }) => {
        rememberInvite(id, token);
        nav.replace({ name: 'open', id });
      })
      .catch((err) => setError(err instanceof ApiError ? err.message : 'Не удалось открыть приглашение'));
  }, [token, nav]);

  if (error) return <Failure message={error} />;
  return <Loading />;
}
