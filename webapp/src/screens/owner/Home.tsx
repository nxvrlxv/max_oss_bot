import { api } from '../../api/client';
import { choiceLabels, type Meeting } from '../../api/types';
import { Badge, Chevron, CheckCircle, Failure, Loading, SectionTitle, VoteScale, useLoad } from '../../components/ui';
import { area, day, percent } from '../../lib/format';
import { useNav, useSecretTaps } from '../../lib/nav';

// Главный экран: собрания, где человек инициатор или подал заявку на квартиру.
export function Home() {
  const nav = useNav();
  const { data, error, loading, reload } = useLoad(() => api.me(), []);
  // Пять быстрых нажатий на заголовок — панель диагностики запуска.
  const tapTitle = useSecretTaps();

  if (loading && !data) return <Loading />;
  if (error || !data) return <Failure message={error ?? 'Не удалось загрузить'} onRetry={reload} />;

  const active = data.meetings.filter((m) => m.status !== 'finished');
  const finished = data.meetings.filter((m) => m.status === 'finished');

  return (
    <div className="screen">
      <div className="header" style={{ paddingBottom: 12, gap: 4 }}>
        <h1 className="h1" onClick={tapTitle}>Голосования</h1>
        <HomeSubtitle meetings={data.meetings} />
      </div>

      {data.meetings.length === 0 && (
        <div className="card" style={{ marginTop: 8 }}>
          <p className="title">Пока пусто</p>
          <p className="caption" style={{ marginTop: 4 }}>
            Откройте голосование по кнопке из чата дома — оно появится здесь. Или начните собрание сами.
          </p>
        </div>
      )}

      {active.length > 0 && (
        <>
          <SectionTitle count={active.length}>Активные</SectionTitle>
          <div className="stack">
            {active.map((meeting) => <ActiveCard key={meeting.id} meeting={meeting} />)}
          </div>
        </>
      )}

      {finished.length > 0 && (
        <>
          <div className="gap" />
          <SectionTitle count={finished.length}>Завершённые</SectionTitle>
          <div className="list">
            {finished.map((meeting) => (
              <button key={meeting.id} type="button" className="list-row"
                onClick={() => nav.go(meeting.is_initiator ? { name: 'dashboard', id: meeting.id } : { name: 'vote', id: meeting.id })}>
                <div className="inner">
                  <div className="body">
                    <div className="strong">{meeting.question}</div>
                    <div className="caption card-note" style={{ marginTop: 0 }}>
                      <span className={`dot ${meeting.summary?.accepted ? 'green' : 'red'}`} />
                      {meeting.summary?.accepted ? 'Принято' : 'Не принято'}
                      {meeting.ends_at && ` · ${day(meeting.ends_at)}`}
                    </div>
                  </div>
                  <Chevron />
                </div>
              </button>
            ))}
          </div>
        </>
      )}

      <div style={{ padding: '24px 12px 0' }}>
        <button type="button" className="btn plain" onClick={() => nav.go({ name: 'create' })}>
          Создать собрание
        </button>
      </div>
    </div>
  );
}

// Подзаголовок: адрес дома и своя квартира, если они однозначны.
function HomeSubtitle({ meetings }: { meetings: Meeting[] }) {
  const addresses = [...new Set(meetings.map((m) => m.address))];
  if (addresses.length !== 1) return null;

  const claims = meetings.flatMap((m) => m.claims);
  const flats = [...new Map(claims.map((c) => [c.flat_number, c])).values()];
  const flatText = flats.length === 1 ? ` · кв. ${flats[0].flat_number}, ${area(flats[0].flat_area)}` : '';

  return <div className="caption">{addresses[0]}{flatText}</div>;
}

function ActiveCard({ meeting }: { meeting: Meeting }) {
  const nav = useNav();
  const deadline = meeting.ends_at ? `до ${day(meeting.ends_at)}` : '';

  if (meeting.is_initiator) {
    const draft = meeting.status === 'draft';
    const summary = meeting.summary ?? { turnout: 0, for: 0, against: 0, abstain: 0, quorum: false, accepted: false };
    // Отметка решения имеет смысл, только если порог считается от всего дома.
    const marks = [{ at: 0.5 }];
    if (meeting.rule.base === 'total' && meeting.rule.threshold !== 0.5) marks.push({ at: meeting.rule.threshold });
    return (
      <button type="button" className="card" onClick={() => nav.go(draft ? { name: 'setup', id: meeting.id } : { name: 'dashboard', id: meeting.id })}>
        <div className="card-top">
          {draft ? <Badge tone="orange">Черновик</Badge> : <Badge>Вы инициатор</Badge>}
          <span className="caption">{deadline}</span>
        </div>
        <div className="card-title">
          <div className="title">{meeting.question}</div>
          <Chevron />
        </div>
        {draft ? (
          <div className="card-note caption">Загрузите реестр и опубликуйте вопрос</div>
        ) : (
          <>
            <VoteScale total={1} tally={summary} marks={marks} />
            <div className="card-note caption">
              {summary.quorum && <CheckCircle size={16} />}
              Проголосовали {percent(summary.turnout)}, {summary.quorum ? 'кворум набран' : 'кворума пока нет'}
            </div>
            {meeting.claims.length > 0 && (
              <div className="card-note caption">
                Ваш голос: {meeting.choice ? choiceLabels[meeting.choice] : 'не отдан'}
              </div>
            )}
          </>
        )}
      </button>
    );
  }

  const pending = meeting.claims.length > 0 && meeting.claims.every((c) => c.status === 'pending');

  if (meeting.choice) {
    return (
      <button type="button" className="card" onClick={() => nav.go({ name: 'vote', id: meeting.id })}>
        <div className="card-top">
          {pending ? <Badge tone="orange">Голос ждёт проверки</Badge> : <Badge tone="green">Вы проголосовали</Badge>}
          <span className="caption">{deadline}</span>
        </div>
        <div className="card-title">
          <div className="title">{meeting.question}</div>
          <Chevron />
        </div>
        <div className="card-note caption">Ваш ответ: {choiceLabels[meeting.choice]}</div>
      </button>
    );
  }

  return (
    <button type="button" className="card" onClick={() => nav.go({ name: 'vote', id: meeting.id })}>
      <div className="card-top">
        <Badge tone="blue">Ждёт вашего голоса</Badge>
        <span className="caption">{deadline}</span>
      </div>
      <div className="card-title">
        <div className="title">{meeting.question}</div>
        <Chevron />
      </div>
      <span className="btn primary">Проголосовать</span>
    </button>
  );
}
