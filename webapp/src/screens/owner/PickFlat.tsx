import { useMemo, useState } from 'react';

import { api, ApiError } from '../../api/client';
import { BottomBar, Failure, InfoIcon, Loading, SectionTitle, useLoad } from '../../components/ui';
import { haptic } from '../../lib/bridge';
import { area, dayTime } from '../../lib/format';
import { useNav } from '../../lib/nav';

// Выбор своей квартиры. ФИО собственников здесь не показываем:
// человек называет квартиру, а инициатор сверяет его с реестром.
export function PickFlat({ id }: { id: number }) {
  const nav = useNav();
  const { data, error, reload } = useLoad(
    () => Promise.all([api.meeting(id), api.flats(id)]),
    [id],
  );
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<string | null>(null);
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

  async function submit() {
    if (!selected) return;
    setSending(true);
    try {
      await api.claim(id, selected);
      haptic.success();
      nav.replace({ name: 'vote', id });
    } catch (err) {
      nav.showError(err instanceof ApiError ? err.message : 'Не удалось отправить заявку');
      setSending(false);
    }
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
            Инициатор сверит заявку с реестром собственников. Проголосовать можно сразу — голос будет учтён после подтверждения.
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
            {sending ? 'Отправляем…' : 'Это моя квартира'}
          </button>
        </BottomBar>
      )}
    </div>
  );
}
