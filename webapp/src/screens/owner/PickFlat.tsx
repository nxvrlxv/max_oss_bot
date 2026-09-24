import { useMemo, useState } from 'react';

import { api, ApiError } from '../../api/client';
import type { Flat, RegistryOwner } from '../../api/types';
import { BottomBar, Failure, InfoIcon, Loading, SectionTitle, useLoad } from '../../components/ui';
import { haptic } from '../../lib/bridge';
import { area, dayTime } from '../../lib/format';
import { useNav } from '../../lib/nav';

// Выбор своей квартиры. Соседу ФИО собственников не показываем:
// он называет квартиру, а инициатор сверяет его с реестром.
// Инициатор реестр и так видит, поэтому сразу выбирает себя в нём —
// и его заявка подтверждается без очереди.
export function PickFlat({ id }: { id: number }) {
  const nav = useNav();
  const { data, error, reload } = useLoad(
    () => Promise.all([api.meeting(id), api.flats(id)]),
    [id],
  );
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<string | null>(null);
  const [owners, setOwners] = useState<RegistryOwner[] | null>(null);
  const [sending, setSending] = useState(false);

  const flats = useMemo(() => {
    if (!data) return [];
    const claimed = new Set(data[0].claims.map((c) => c.flat_number));
    const q = query.trim().toLowerCase();
    return data[1].filter((f) => !claimed.has(f.number) && (!q || f.number.toLowerCase().startsWith(q)));
  }, [data, query]);

  if (error) return <Failure message={error} onRetry={reload} />;
  if (!data) return <Loading />;

  const [meeting] = data;
  const chosen = data[1].find((f) => f.number === selected);

  if (meeting.status !== 'active') {
    return <Failure message="Голосование сейчас не идёт" />;
  }

  function fail(err: unknown, fallback: string) {
    nav.showError(err instanceof ApiError ? err.message : fallback);
    setSending(false);
  }

  async function submit() {
    if (!selected) return;
    setSending(true);
    try {
      if (meeting.is_initiator) {
        setOwners(await api.flatOwners(id, selected));
        setSending(false);
        return;
      }
      await api.claim(id, selected);
      haptic.success();
      nav.replace({ name: 'vote', id });
    } catch (err) {
      fail(err, 'Не удалось отправить заявку');
    }
  }

  if (owners && chosen) {
    return (
      <OwnerStep flat={chosen} owners={owners} sending={sending}
        onBack={() => setOwners(null)}
        onConfirm={async (ownerId) => {
          setSending(true);
          try {
            await api.claim(id, chosen.number, ownerId);
            haptic.success();
            nav.replace({ name: 'vote', id });
          } catch (err) {
            fail(err, 'Не удалось сохранить');
          }
        }} />
    );
  }

  return (
    <div className="screen with-bar">
      <div className="header">
        <div className="caption">Вопрос собрания · {meeting.address}</div>
        <h1 className="h1">{meeting.question}</h1>
        {meeting.ends_at && <div className="meta caption">Голосование до {dayTime(meeting.ends_at)}</div>}
      </div>

      <div className="banner">
        <InfoIcon />
        <div className="body">
          <div className="strong">Выберите свою квартиру</div>
          <div className="caption">
            {meeting.is_initiator
              ? 'Затем укажите себя в реестре — ваш голос будет учтён сразу.'
              : 'Инициатор сверит заявку с реестром собственников. Проголосовать можно сразу — голос будет учтён после подтверждения.'}
          </div>
        </div>
      </div>

      <div style={{ padding: '16px 12px 12px' }}>
        <input className="input" inputMode="text" placeholder="Номер квартиры или помещения"
          value={query} onChange={(e) => setQuery(e.target.value)} aria-label="Поиск квартиры" />
      </div>

      <SectionTitle count={flats.length}>Помещения</SectionTitle>
      <div className="flat-grid">
        {flats.map((flat) => (
          <button key={flat.id} type="button" className="flat" aria-pressed={flat.number === selected}
            onClick={() => { haptic.select(); setSelected(flat.number); }}>
            {flat.number}
          </button>
        ))}
      </div>
      {flats.length === 0 && <p className="caption" style={{ padding: '8px 28px' }}>Такого номера в реестре нет</p>}

      {chosen && (
        <BottomBar hint={`Кв. ${chosen.number} · ${area(chosen.area)}`}>
          <button type="button" className="btn primary large" disabled={sending} onClick={submit}>
            {sending ? 'Секунду…' : meeting.is_initiator ? 'Далее' : 'Это моя квартира'}
          </button>
        </BottomBar>
      )}
    </div>
  );
}

// Шаг инициатора: кто он в реестре этой квартиры.
function OwnerStep({ flat, owners, sending, onBack, onConfirm }: {
  flat: Flat;
  owners: RegistryOwner[];
  sending: boolean;
  onBack: () => void;
  onConfirm: (ownerId: number) => void;
}) {
  const free = owners.filter((o) => !o.taken);
  const [ownerId, setOwnerId] = useState<number | null>(free.length === 1 ? free[0].id : null);

  return (
    <div className="screen with-bar">
      <div className="header">
        <div className="caption">Кв. {flat.number} · {area(flat.area)}</div>
        <h1 className="h1">Кто вы по реестру?</h1>
        <div className="caption">Голос будет учтён с долей этого собственника</div>
      </div>

      <div className="options">
        {owners.map((owner) => (
          <button key={owner.id} type="button" className="option" aria-pressed={ownerId === owner.id}
            disabled={owner.taken} style={{ opacity: owner.taken ? 0.5 : 1 }}
            onClick={() => { haptic.select(); setOwnerId(owner.id); }}>
            <span className="radio" />
            <span className="desc">
              <span className="strong">{owner.name || 'Без имени'}</span>
              <span className="caption">{owner.taken ? 'уже подтверждён за другим человеком' : `доля ${area(owner.owned_area)}`}</span>
            </span>
          </button>
        ))}
      </div>

      <button type="button" className="btn link" style={{ marginTop: 8 }} onClick={onBack}>
        Другая квартира
      </button>

      <BottomBar>
        <button type="button" className="btn primary large" disabled={sending || ownerId === null}
          onClick={() => ownerId !== null && onConfirm(ownerId)}>
          {sending ? 'Сохраняем…' : 'Это я'}
        </button>
      </BottomBar>
    </div>
  );
}
