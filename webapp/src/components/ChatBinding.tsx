import { useState } from 'react';

import { api, ApiError } from '../api/client';
import type { Meeting } from '../api/types';
import { copyText } from '../lib/bridge';

export function ChatBinding({ meeting, onRefresh }: { meeting: Meeting; onRefresh: () => void }) {
  const [message, setMessage] = useState('');
  const [checking, setChecking] = useState(false);
  const [checkFailed, setCheckFailed] = useState(false);
  const command = `/bind ${meeting.id}`;

  async function copy() {
    setCheckFailed(false);
    const result = await copyText(command);
    setMessage(result === 'copied' ? 'Команда скопирована — отправьте её в чат дома от своего аккаунта'
      : 'Выделите команду и скопируйте вручную');
  }

  async function check() {
    if (checking) return;
    setChecking(true);
    setMessage('');
    setCheckFailed(false);
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 15_000);
    try {
      const current = await api.meeting(meeting.id, controller.signal);
      setMessage(current.chat_bound
        ? 'Проверено: домовой чат привязан к этому собранию.'
        : 'Привязка пока не найдена. Назначьте бота администратором чата с правом чтения сообщений, затем отправьте команду от аккаунта, создавшего собрание. Эта кнопка только проверяет результат.');
      onRefresh();
    } catch (err) {
      setCheckFailed(true);
      setMessage(controller.signal.aborted ? 'Сервер не ответил за 15 секунд. Попробуйте проверить ещё раз.'
        : err instanceof ApiError ? err.message : 'Не удалось проверить привязку. Попробуйте ещё раз.');
    } finally {
      window.clearTimeout(timeout);
      setChecking(false);
    }
  }

  return (
    <div className="card">
      <div className="strong">{meeting.chat_bound ? 'Домовой чат привязан' : 'Привязать домовой чат'}</div>
      <div className="caption" style={{ marginTop: 4 }}>
        {meeting.chat_bound
          ? 'Для повторной отправки объявления отправьте команду в привязанный чат.'
          : 'Добавьте бота в нужный чат и назначьте администратором с правом чтения сообщений. Затем отправьте туда эту команду от аккаунта, создавшего собрание.'}
        {' '}{meeting.status === 'draft'
          ? 'Вопрос, ссылка и QR-код появятся в чате после публикации собрания.'
          : 'Бот сразу отправит вопрос, ссылку и QR-код голосования.'}
      </div>
      <div className="invite-link">{command}</div>
      <div className="btn-row" style={{ marginTop: 12 }}>
        <button type="button" className="btn secondary" onClick={copy}>Скопировать команду</button>
        <button type="button" className="btn secondary" disabled={checking} aria-busy={checking} onClick={check}>
          {checking ? 'Проверяем…' : 'Проверить привязку'}
        </button>
      </div>
      {message && <div className="caption" role={checkFailed ? 'alert' : 'status'}>{message}</div>}
    </div>
  );
}
