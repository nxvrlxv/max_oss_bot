import { useState } from 'react';

import { api, ApiError } from '../api/client';
import type { Claim } from '../api/types';
import { haptic } from '../lib/bridge';
import { useNav } from '../lib/nav';

// «Не моя квартира»: снять ошибочный выбор. Подтверждённую квартиру
// снимает только инициатор (подтверждал её он сам), и вместе с ней из
// итога уходит его голос по этой доле — поэтому сначала переспрашиваем.
export function CancelFlat({ meetingId, claim, onDone }: {
  meetingId: number;
  claim: Claim;
  onDone: () => void;
}) {
  const nav = useNav();
  const [asking, setAsking] = useState(false);
  const [busy, setBusy] = useState(false);
  const confirmed = claim.status === 'confirmed';

  async function cancel() {
    setBusy(true);
    try {
      await api.cancelClaim(meetingId, claim.id);
      haptic.success();
      onDone();
    } catch (err) {
      nav.showError(err instanceof ApiError ? err.message : 'Не удалось снять квартиру');
    } finally {
      setBusy(false);
      setAsking(false);
    }
  }

  if (!asking) {
    return (
      <button type="button" className="btn danger" style={{ width: 'auto' }} onClick={() => setAsking(true)}>
        Кв. {claim.flat_number} — не моя квартира
      </button>
    );
  }

  return (
    <div className="banner red" style={{ margin: '8px 0', width: '100%' }}>
      <div className="body" style={{ flexGrow: 1 }}>
        <div className="strong">{confirmed ? `Снять кв. ${claim.flat_number}?` : `Отозвать заявку на кв. ${claim.flat_number}?`}</div>
        <div className="caption">
          {confirmed
            ? 'Ваш голос по этой квартире уйдёт из подсчёта, выбор собственника сбросится. Потом можно выбрать квартиру заново.'
            : 'Инициатор её больше не увидит.'}
        </div>
        <div className="btn-row" style={{ marginTop: 12 }}>
          <button type="button" className="btn secondary" disabled={busy} onClick={() => setAsking(false)}>Оставить</button>
          <button type="button" className="btn primary" style={{ background: 'var(--icon-negative)' }} disabled={busy} onClick={cancel}>
            {busy ? 'Снимаем…' : 'Снять'}
          </button>
        </div>
      </div>
    </div>
  );
}
