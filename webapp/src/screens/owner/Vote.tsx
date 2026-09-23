import { useEffect, useState, type CSSProperties } from 'react';

import { api, ApiError } from '../../api/client';
import { choiceLabels, type Choice, type Meeting } from '../../api/types';
import { BottomBar, CheckCircle, Failure, InfoIcon, Loading, SectionTitle, useLoad } from '../../components/ui';
import { haptic } from '../../lib/bridge';
import { area, dayTime } from '../../lib/format';
import { useNav } from '../../lib/nav';

const choices: Choice[] = ['for', 'against', 'abstain'];

// Голосование собственника: выбор → подтверждение → «спасибо» → «вы уже проголосовали».
export function Vote({ id }: { id: number }) {
  const nav = useNav();
  const { data: meeting, setData, error, reload } = useLoad(() => api.meeting(id), [id]);
  const [picked, setPicked] = useState<Choice | null>(null);
  const [editing, setEditing] = useState(false);
  const [sending, setSending] = useState(false);
  const [thanks, setThanks] = useState<Choice | null>(null);

  // Без заявки голосовать нечем — сначала выбор квартиры.
  useEffect(() => {
    if (meeting && meeting.claims.length === 0 && meeting.status === 'active') {
      nav.replace({ name: 'pick', id });
    }
  }, [meeting, id, nav]);

  if (error) return <Failure message={error} onRetry={reload} />;
  if (!meeting) return <Loading />;

  const open = meeting.status === 'active';
  const pending = meeting.claims.filter((c) => c.status === 'pending');
  const allPending = meeting.claims.length > 0 && pending.length === meeting.claims.length;
  const choosing = open && (!meeting.choice || editing);

  async function confirm() {
    if (!picked) return;
    setSending(true);
    try {
      const updated = await api.vote(id, picked);
      haptic.success();
      setData(updated);
      setThanks(picked);
      setEditing(false);
      setPicked(null);
    } catch (err) {
      nav.showError(err instanceof ApiError ? err.message : 'Не удалось сохранить голос');
    } finally {
      setSending(false);
    }
  }

  async function cancelClaim(claimId: number) {
    try {
      await api.cancelClaim(id, claimId);
      const updated = await api.meeting(id);
      setData(updated);
    } catch (err) {
      nav.showError(err instanceof ApiError ? err.message : 'Не удалось отозвать заявку');
    }
  }

  if (thanks) {
    return <Thanks choice={thanks} counted={!allPending} onDone={() => setThanks(null)} />;
  }

  return (
    <div className={`screen ${picked ? 'with-bar' : ''}`}>
      <div className="header" style={{ paddingBottom: 20 }}>
        <div className="caption">Вопрос собрания · {meeting.address}</div>
        <h1 className="h1">{meeting.question}</h1>
        <div className="meta caption">
          <WeightLine meeting={meeting} />
          {meeting.ends_at && (
            <span>{open ? 'Голосование до' : 'Голосование завершилось'} {dayTime(meeting.ends_at)}</span>
          )}
        </div>
      </div>

      {pending.length > 0 && open && (
        <>
          <div className="banner orange">
            <InfoIcon color="var(--warning)" />
            <div className="body">
              <div className="strong">
                {pending.length === 1 ? `Заявка на кв. ${pending[0].flat_number} на проверке` : 'Заявки на проверке'}
              </div>
              <div className="caption">
                Инициатор сверит её с реестром собственников. Голос сохранится и будет учтён после подтверждения.
              </div>
            </div>
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center' }}>
            {pending.map((claim) => (
              <button key={claim.id} type="button" className="btn link" style={{ width: 'auto' }}
                onClick={() => cancelClaim(claim.id)}>
                Кв. {claim.flat_number} — не моя квартира
              </button>
            ))}
          </div>
        </>
      )}

      {meeting.status === 'finished' && <FinishedResult meeting={meeting} />}

      {!choosing && meeting.choice && (
        <>
          {/* Пока заявка на проверке, о голосе уже сказано в плашке выше. */}
          {open && !allPending && <VotedCard meeting={meeting} />}
          <SectionTitle>Ваш ответ</SectionTitle>
          <div className="options">
            <div className="option static" aria-pressed="true">
              <span className="radio" />
              <span className="title">{choiceLabels[meeting.choice]}</span>
            </div>
          </div>
          {open && (
            <button type="button" className="btn link" onClick={() => { setEditing(true); setPicked(meeting.choice ?? null); }}>
              Изменить ответ
            </button>
          )}
        </>
      )}

      {choosing && (
        <>
          <SectionTitle>Ваш ответ</SectionTitle>
          <div className="options">
            {choices.map((choice) => (
              <button key={choice} type="button" className="option" aria-pressed={picked === choice}
                onClick={() => { haptic.select(); setPicked(choice); }}>
                <span className="radio" />
                <span className="title">{choiceLabels[choice]}</span>
              </button>
            ))}
          </div>
        </>
      )}

      {open && !picked && (
        <button type="button" className="btn link" style={{ marginTop: 8 }} onClick={() => nav.go({ name: 'pick', id })}>
          У меня ещё одна квартира в этом доме
        </button>
      )}

      {picked && choosing && (
        <BottomBar>
          <button type="button" className="btn primary large" disabled={sending || picked === meeting.choice}
            onClick={confirm}>
            {sending ? 'Сохраняем…' : 'Подтвердить выбор'}
          </button>
        </BottomBar>
      )}
    </div>
  );
}

// «Ваш голос: квартира 14 · 54,2 м²». До подтверждения вес ещё неизвестен —
// показываем площадь квартиры.
function WeightLine({ meeting }: { meeting: Meeting }) {
  if (meeting.claims.length === 0) return null;
  const numbers = meeting.claims.map((c) => c.flat_number);
  const weight = meeting.claims.reduce((sum, c) => sum + (c.status === 'confirmed' ? c.weight : c.flat_area), 0);
  const label = numbers.length === 1 ? `квартира ${numbers[0]}` : `квартиры ${numbers.join(', ')}`;
  return <span>Ваш голос: {label} · {area(weight)}</span>;
}

function VotedCard({ meeting }: { meeting: Meeting }) {
  return (
    <div className="banner green" style={{ marginBottom: 20, alignItems: 'center', padding: 16 }}>
      <CheckCircle size={32} />
      <div className="body">
        <div className="title">Вы уже проголосовали</div>
        <div className="caption">Ваш голос учтён {dayTime(meeting.voted_at)}</div>
      </div>
    </div>
  );
}

function FinishedResult({ meeting }: { meeting: Meeting }) {
  const accepted = meeting.summary?.accepted;
  return (
    <div className={`banner ${accepted ? 'green' : 'red'}`} style={{ marginBottom: 20, padding: 16 }}>
      <div className="body">
        <div className="title">{accepted ? 'Решение принято' : 'Решение не принято'}</div>
        <div className="caption">
          {meeting.summary?.quorum ? 'Кворум был набран' : 'Кворум не набран — собрание неправомочно'}
        </div>
      </div>
    </div>
  );
}

// Экран благодарности из макета, с той же анимацией.
function Thanks({ choice, counted, onDone }: { choice: Choice; counted: boolean; onDone: () => void }) {
  const dots = [[0, -86], [61, -61], [86, 0], [61, 61], [0, 86], [-61, 61], [-86, 0], [-61, -61]];
  return (
    <div className="thanks">
      <div className="content">
        <svg width="200" height="200" viewBox="0 0 200 200" aria-hidden="true" style={{ display: 'block' }}>
          {dots.map(([dx, dy], i) => (
            <circle key={i} className="mx-dot" cx="100" cy="100" r="5" fill={i % 2 ? '#007aff' : '#2bc644'}
              style={{ '--dx': `${dx}px`, '--dy': `${dy}px` } as CSSProperties} />
          ))}
          <circle className="mx-pop" cx="100" cy="100" r="56" fill="#dff6e4" />
          <circle className="mx-ring" cx="100" cy="100" r="56" fill="none" stroke="#2bc644" strokeWidth="6"
            strokeLinecap="round" transform="rotate(-90 100 100)" />
          <path className="mx-check" d="M72 102 L92 122 L130 78" fill="none" stroke="#2bc644" strokeWidth="9"
            strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        <h1 className="h1 mx-fade" style={{ marginTop: 16 }}>Спасибо за ваш голос!</h1>
        <p className="text muted mx-fade" style={{ maxWidth: 300 }}>
          {counted
            ? `Ответ «${choiceLabels[choice]}» записан и учтён в итогах собрания`
            : `Ответ «${choiceLabels[choice]}» сохранён. Он будет учтён, когда инициатор подтвердит вашу квартиру`}
        </p>
      </div>
      <div className="actions">
        <button type="button" className="btn secondary large" onClick={onDone}>Готово</button>
      </div>
    </div>
  );
}
