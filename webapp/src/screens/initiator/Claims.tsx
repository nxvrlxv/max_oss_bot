import { useState } from 'react';

import { api, ApiError } from '../../api/client';
import type { PendingClaim } from '../../api/types';
import { Failure, Loading, useLoad } from '../../components/ui';
import { haptic } from '../../lib/bridge';
import { area, dayTime } from '../../lib/format';
import { useNav } from '../../lib/nav';

// Очередь заявок: человек из MAX против собственников квартиры по реестру.
export function Claims({ id }: { id: number }) {
  const [tab, setTab] = useState<'pending' | 'confirmed'>('pending');
  const { data, error, loading, reload } = useLoad(() => tab === 'pending' ? api.pendingClaims(id) : api.confirmedClaims(id), [id, tab], 15_000);

  if (error) return <Failure message={error} onRetry={reload} />;
  if (!data || loading) return <Loading />;

  return (
    <div className="screen">
      <div className="header">
        <h1 className="h1">Заявки</h1>
        <div className="caption">
          Сверьте имя в MAX с собственниками по реестру. Если сомневаетесь — уточните у соседа лично.
        </div>
      </div>

      <div className="btn-row" style={{ marginBottom: 12 }}>
        <button className={`btn ${tab === 'pending' ? 'primary' : 'secondary'}`} onClick={() => setTab('pending')}>На проверке</button>
        <button className={`btn ${tab === 'confirmed' ? 'primary' : 'secondary'}`} onClick={() => setTab('confirmed')}>Подтверждённые</button>
      </div>

      {data.length === 0 ? (
        <div className="card"><p className="title">{tab === 'pending' ? 'Нет заявок на проверке' : 'Нет подтверждённых заявок'}</p></div>
      ) : (
        <div className="stack">
          {data.map((claim) => <ClaimCard key={`${claim.id}-${claim.status}`} meetingId={id} claim={claim} onDone={reload} />)}
        </div>
      )}
    </div>
  );
}

function ClaimCard({ meetingId, claim, onDone }: { meetingId: number; claim: PendingClaim; onDone: () => void }) {
  const nav = useNav();
  const [busy, setBusy] = useState(false);
  const [revoking, setRevoking] = useState(false);

  async function act(action: () => Promise<void>) {
    setBusy(true);
    try {
      await action();
      haptic.success();
      onDone();
    } catch (err) {
      nav.showError(err instanceof ApiError ? err.message : 'Не удалось сохранить');
      setBusy(false);
    }
  }

  if (claim.status === 'confirmed') return (
    <div className="card">
      <div className="title">Кв. {claim.flat_number} · {claim.owner_name}</div>
      <div className="caption">{claim.user_name} в MAX · подтверждённая доля {area(claim.weight)}</div>
      {revoking ? <>
        <p className="caption">Отменить подтверждение? Голос по этой доле выйдет из подсчёта, собственник освободится, заявка вернётся на проверку.</p>
        <div className="btn-row">
          <button className="btn secondary" disabled={busy} onClick={() => setRevoking(false)}>Оставить</button>
          <button className="btn danger" disabled={busy} onClick={() => act(() => api.revokeClaim(meetingId, claim.id))}>Отменить подтверждение</button>
        </div>
      </> : <button className="btn danger" disabled={busy} onClick={() => setRevoking(true)}>Отменить подтверждение</button>}
    </div>
  );

  return (
    <div className="card">
      <div className="card-top">
        <div className="title">Кв. {claim.flat_number}</div>
        <span className="caption">{area(claim.flat_area)}</span>
      </div>
      <div className="strong" style={{ marginTop: 8 }}>{claim.user_name}</div>
      <div className="caption">в MAX · заявка {dayTime(claim.created_at)}</div>
      {claim.requested_owner_id && <div className="strong" style={{ marginTop: 8 }}>Выбрал: {claim.owner_name} · доля {area(claim.weight)}</div>}
      {claim.confirmed_as && (
        <div className="caption" style={{ marginTop: 4 }}>Уже подтверждён как «{claim.confirmed_as}»</div>
      )}

      {!claim.requested_owner_id && (
        <p className="caption" style={{ marginTop: 8 }}>
          В старой заявке не выбран собственник. Отклоните её и попросите участника выбрать себя и проголосовать заново.
        </p>
      )}
      <div className="btn-row" style={{ marginTop: 12 }}>
        <button type="button" className="btn secondary" disabled={busy}
          onClick={() => act(() => api.rejectClaim(meetingId, claim.id))}>
          Отклонить
        </button>
        <button type="button" className="btn primary" disabled={busy || !claim.requested_owner_id}
          onClick={() => act(() => api.confirmClaim(meetingId, claim.id))}>
          Подтвердить
        </button>
      </div>
    </div>
  );
}
