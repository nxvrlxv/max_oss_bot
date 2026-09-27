import { useState } from 'react';

import type { Meeting } from '../api/types';
import { copyText } from '../lib/bridge';

export function ChatBinding({ meeting, onRefresh }: { meeting: Meeting; onRefresh: () => void }) {
  const [message, setMessage] = useState('');
  const command = `/bind ${meeting.id}`;

  async function copy() {
    const result = await copyText(command);
    setMessage(result === 'copied' ? 'Команда скопирована — отправьте её в чат дома от своего аккаунта'
      : 'Выделите команду и скопируйте вручную');
  }

  return (
    <div className="card">
      <div className="strong">{meeting.chat_bound ? 'Домовой чат привязан' : 'Привязать домовой чат'}</div>
      <div className="caption" style={{ marginTop: 4 }}>
        {meeting.chat_bound
          ? 'Для повторной отправки объявления отправьте команду в привязанный чат.'
          : 'Добавьте бота в нужный чат, если его ещё нет, и отправьте туда эту команду от своего аккаунта инициатора.'}
        {' '}{meeting.status === 'draft'
          ? 'Вопрос, ссылка и QR-код появятся в чате после публикации собрания.'
          : 'Бот сразу отправит вопрос, ссылку и QR-код голосования.'}
      </div>
      <div className="invite-link">{command}</div>
      <div className="btn-row" style={{ marginTop: 12 }}>
        <button type="button" className="btn secondary" onClick={copy}>Скопировать команду</button>
        <button type="button" className="btn secondary" onClick={onRefresh}>Проверить привязку</button>
      </div>
      {message && <div className="caption" role="status">{message}</div>}
    </div>
  );
}
