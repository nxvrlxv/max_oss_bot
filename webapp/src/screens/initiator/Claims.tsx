import { useState } from 'react';

import { api, ApiError } from '../../api/client';
import type { PendingClaim } from '../../api/types';
import { Failure, Loading, useLoad } from '../../components/ui';
import { haptic } from '../../lib/bridge';
import { area, dayTime, samePerson } from '../../lib/format';
import { useNav } from '../../lib/nav';

// Очередь заявок: человек из MAX против собственников квартиры по реестру.
export function Claims({ id }: { id: number }) {
  const { data, setData, error, reload } = useLoad(() => api.pendingClaims(id), [id], 15_000);

  if (error) return <Failure message={error} onRetry={reload} />;
  if (!data) return <Loading />;

  const remove = (claimId: number) => setData(data.filter((c) => c.id !== claimId));

  return (
    <div className="screen">
      <div className="header">
        <h1 className="h1">Заявки</h1>
        <div className="caption">
          Сверьте имя в MAX с собственниками по реестру. Если сомневаетесь — уточните у соседа лично.
        </div>
      </div>

      {data.length === 0 ? (
        <div className="card"><p className="title">Все заявки разобраны</p></div>
      ) : (
        <div className="stack">
          {data.map((claim) => <ClaimCard key={claim.id} meetingId={id} claim={claim} onDone={() => remove(claim.id)} />)}
        </div>
      )}
    </div>
  );
}

function ClaimCard({ meetingId, claim, onDone }: { meetingId: number; claim: PendingClaim; onDone: () => void }) {
  const nav = useNav();
  const owners = claim.owners ?? [];
  // Уже подтверждённый по другой квартире человек — это тот же собственник:
  // доли с другим ФИО ему не подходят, сервер их и не примет.
  const fits = (name: string) => !claim.confirmed_as || samePerson(claim.confirmed_as, name);
  const free = owners.filter((o) => !o.taken && fits(o.name));
  const [ownerId, setOwnerId] = useState<number | null>(free.length === 1 ? free[0].id : null);
  const [busy, setBusy] = useState(false);

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

  return (
    <div className="card">
      <div className="card-top">
        <div className="title">Кв. {claim.flat_number}</div>
        <span className="caption">{area(claim.flat_area)}</span>
      </div>
      <div className="strong" style={{ marginTop: 8 }}>{claim.user_name}</div>
      <div className="caption">в MAX · заявка {dayTime(claim.created_at)}</div>
      {claim.confirmed_as && (
        <div className="caption" style={{ marginTop: 4 }}>Уже подтверждён как «{claim.confirmed_as}»</div>
      )}

      <div className="divider" style={{ margin: '12px 0' }} />
      <div className="caption" style={{ marginBottom: 8 }}>Кто это по реестру?</div>
      <div className="stack" style={{ gap: 8 }}>
        {owners.map((owner) => {
          const blocked = owner.taken || !fits(owner.name);
          return (
            <button key={owner.id} type="button" className="option" aria-pressed={ownerId === owner.id}
              disabled={blocked} style={{ minHeight: 56, opacity: blocked ? 0.5 : 1 }}
              onClick={() => setOwnerId(owner.id)}>
              <span className="radio" />
              <span className="desc">
                <span className="strong">{owner.name || 'Без имени'}</span>
                <span className="caption">
                  {owner.taken ? 'уже подтверждён за другим человеком'
                    : !fits(owner.name) ? 'другое ФИО — не этот человек'
                    : `доля ${area(owner.owned_area)}`}
                </span>
              </span>
            </button>
          );
        })}
      </div>
      {free.length === 0 && (
        <p className="caption" style={{ marginTop: 8 }}>
          Подходящего собственника нет — заявку остаётся отклонить
        </p>
      )}

      <div className="btn-row" style={{ marginTop: 12 }}>
        <button type="button" className="btn secondary" disabled={busy}
          onClick={() => act(() => api.rejectClaim(meetingId, claim.id))}>
          Отклонить
        </button>
        <button type="button" className="btn primary" disabled={busy || ownerId === null}
          onClick={() => ownerId !== null && act(() => api.confirmClaim(meetingId, claim.id, ownerId))}>
          Подтвердить
        </button>
      </div>
    </div>
  );
}
