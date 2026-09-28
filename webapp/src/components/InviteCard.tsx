import { useState } from 'react';

import { haptic, shareLink, copyText, downloadFile, type ShareResult } from '../lib/bridge';

type Status = ShareResult | 'downloaded' | 'not-downloaded';

const messages: Record<Status, string> = {
  copied: 'Ссылка скопирована',
  shared: 'Готово',
  failed: 'Не получилось — выделите ссылку и скопируйте вручную',
  downloaded: 'QR-код сохранён — распечатайте его для подъезда',
  'not-downloaded': 'Не получилось скачать — сделайте снимок экрана с QR-кодом',
};

// Ссылка на голосование: переслать соседям, которых нет в чате дома,
// а QR с той же ссылкой — распечатать и повесить в подъезде.
export function InviteCard({ link, qr, question }: { link: string; qr?: string; question: string }) {
  const [status, setStatus] = useState<Status | null>(null);

  async function run(action: () => Promise<Status>) {
    const outcome = await action();
    setStatus(outcome);
    if (outcome !== 'failed' && outcome !== 'not-downloaded') haptic.success();
  }

  const text = `Голосование собственников: «${question}». Откройте по ссылке и выберите свою квартиру`;

  return (
    <div className="card">
      <div className="strong">Ссылка на голосование</div>
      <div className="caption" style={{ marginTop: 4 }}>
        Для соседей, которых нет в чате дома: откроет голосование сразу на выборе квартиры
      </div>
      {qr && <img className="invite-qr" src={qr} width={200} height={200} alt="QR-код со ссылкой на голосование" />}
      <div className="invite-link">{link}</div>
      <div className="btn-row" style={{ marginTop: 12 }}>
        <button type="button" className="btn secondary" onClick={() => run(() => copyText(link))}>
          Скопировать
        </button>
        <button type="button" className="btn primary" onClick={() => run(() => shareLink(link, text))}>
          Поделиться
        </button>
      </div>
      {qr && (
        <button type="button" className="btn secondary"
          onClick={() => run(async () => (await downloadFile(qr, 'qr-golosovanie.png')) ? 'downloaded' : 'not-downloaded')}>
          Скачать QR-код для подъезда
        </button>
      )}
      {status && (
        <div className="caption" role="status" style={{ marginTop: 8, textAlign: 'center' }}>
          {messages[status]}
        </div>
      )}
    </div>
  );
}
