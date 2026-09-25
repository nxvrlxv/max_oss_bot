import { useState } from 'react';

import { api, ApiError } from '../api/client';
import type { Meeting } from '../api/types';
import { haptic } from '../lib/bridge';
import { plural } from '../lib/format';
import { useNav } from '../lib/nav';

// Удаление голосования инициатором. Действие необратимое, поэтому сначала
// показываем, что именно пропадёт. Завершённое собрание сервер удалить
// не даст — его итог нужен для протокола, — и кнопку для него не рисуем.
export function DeleteMeeting({ meeting, votes = 0 }: { meeting: Meeting; votes?: number }) {
  const nav = useNav();
  const [asking, setAsking] = useState(false);
  const [busy, setBusy] = useState(false);

  if (meeting.status === 'finished') return null;
  const draft = meeting.status === 'draft';

  async function remove() {
    setBusy(true);
    try {
      await api.deleteMeeting(meeting.id);
      haptic.success();
      nav.reset({ name: 'home' });
      nav.showNotice(draft ? 'Черновик удалён' : 'Голосование удалено');
    } catch (err) {
      nav.showError(err instanceof ApiError ? err.message : 'Не удалось удалить');
      setBusy(false);
    }
  }

  if (!asking) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', marginTop: 24 }}>
        <button type="button" className="btn danger" style={{ width: 'auto' }} onClick={() => setAsking(true)}>
          {draft ? 'Удалить черновик' : 'Удалить голосование'}
        </button>
      </div>
    );
  }

  return (
    <div className="banner red" style={{ marginTop: 24 }}>
      <div className="body" style={{ flexGrow: 1 }}>
        <div className="strong">{draft ? 'Удалить черновик?' : 'Удалить голосование?'}</div>
        {draft ? (
          <div className="caption">Черновик и загруженный реестр будут удалены.</div>
        ) : (
          <>
            <div className="caption">
              {votes > 0
                ? `Уже ${plural(votes, ['отдан', 'отданы', 'отдано'])} ${votes} ${plural(votes, ['голос', 'голоса', 'голосов'])} — они будут удалены безвозвратно вместе с заявками и реестром.`
                : 'Голосов пока нет. Заявки и реестр будут удалены.'}
            </div>
            {meeting.chat_bound && <div className="caption">В чат дома уйдёт сообщение об отмене.</div>}
          </>
        )}
        <div className="btn-row" style={{ marginTop: 12 }}>
          <button type="button" className="btn secondary" disabled={busy} onClick={() => setAsking(false)}>Оставить</button>
          <button type="button" className="btn primary" style={{ background: 'var(--icon-negative)' }} disabled={busy} onClick={remove}>
            {busy ? 'Удаляем…' : 'Удалить навсегда'}
          </button>
        </div>
      </div>
    </div>
  );
}
