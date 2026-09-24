import { useState } from 'react';

import { haptic, shareLink, copyText, type ShareResult } from '../lib/bridge';

// Ссылка на голосование: переслать соседям, которых нет в чате дома.
// Та же ссылка потом попадёт в QR на объявлении в подъезде.
export function InviteCard({ link, question }: { link: string; question: string }) {
  const [result, setResult] = useState<ShareResult | null>(null);

  async function run(action: () => Promise<ShareResult>) {
    const outcome = await action();
    setResult(outcome);
    if (outcome !== 'failed') haptic.success();
  }

  const text = `Голосование собственников: «${question}». Откройте по ссылке и выберите свою квартиру`;

  return (
    <div className="card">
      <div className="strong">Ссылка на голосование</div>
      <div className="caption" style={{ marginTop: 4 }}>
        Для соседей, которых нет в чате дома: откроет голосование сразу на выборе квартиры
      </div>
      <div className="invite-link">{link}</div>
      <div className="btn-row" style={{ marginTop: 12 }}>
        <button type="button" className="btn secondary" onClick={() => run(() => copyText(link))}>
          Скопировать
        </button>
        <button type="button" className="btn primary" onClick={() => run(() => shareLink(link, text))}>
          Поделиться
        </button>
      </div>
      {result && (
        <div className="caption" role="status" style={{ marginTop: 8, textAlign: 'center' }}>
          {result === 'copied' && 'Ссылка скопирована'}
          {result === 'shared' && 'Готово'}
          {result === 'failed' && 'Не получилось — выделите ссылку и скопируйте вручную'}
        </div>
      )}
    </div>
  );
}
