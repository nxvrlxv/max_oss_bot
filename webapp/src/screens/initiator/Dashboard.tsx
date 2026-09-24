import { useState } from 'react';

import { api } from '../../api/client';
import { choiceLabels, type Meeting, type Threshold } from '../../api/types';
import { InviteCard } from '../../components/InviteCard';
import { Badge, BottomBar, CheckCircle, Failure, Loading, ProgressRing, SectionTitle, VoteScale, useLoad } from '../../components/ui';
import { area, dayTime, initials, percent, shortName } from '../../lib/format';
import { useNav } from '../../lib/nav';

const PAGE = 7;

// Ход голосования для инициатора: явка, оба порога, кого не хватает.
export function Dashboard({ id }: { id: number }) {
  const nav = useNav();
  const { data, error, reload } = useLoad(() => Promise.all([api.meeting(id), api.dashboard(id)]), [id]);
  const [shown, setShown] = useState(PAGE);

  if (error) return <Failure message={error} onRetry={reload} />;
  if (!data) return <Loading />;

  const [meeting, board] = data;
  const notVoted = board.not_voted ?? [];
  const total = board.total_area;
  const open = meeting.status === 'active';

  const marks: { at: number; label: string; caption: string; side?: 'left' | 'right' }[] = [
    { at: 0.5, label: 'Кворум', caption: '50% голосов', side: 'left' },
  ];
  // Порог решения на шкале дома — только если он считается от всего дома.
  if (meeting.rule.base === 'total') {
    marks.push({ at: meeting.rule.threshold, label: meeting.rule.label, caption: `${percent(meeting.rule.threshold)} голосов` });
  }

  return (
    <div className={`screen ${board.pending_claims > 0 ? 'with-bar' : ''}`}>
      <div className="header">
        <div className="caption">Вопрос собрания · {meeting.address}</div>
        <h1 className="h1">{meeting.question}</h1>
        <div className="row">
          {open
            ? <Badge tone="green" tall>Идёт голосование</Badge>
            : <Badge tone={board.accepted ? 'green' : 'red'} tall>{board.accepted ? 'Решение принято' : 'Решение не принято'}</Badge>}
          {meeting.ends_at && <span className="caption">{open ? 'до' : 'завершено'} {dayTime(meeting.ends_at)}</span>}
        </div>
      </div>

      <div className="card">
        <div style={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', gap: 12 }}>
          <div>
            <div className="caption">Проголосовали</div>
            <div className="big">{percent(total > 0 ? board.tally.total / total : 0)}</div>
          </div>
          <div style={{ textAlign: 'right' }}>
            <div className="strong">{board.flats_voted} из {board.flats_total}</div>
            <div className="caption">помещений</div>
          </div>
        </div>

        <VoteScale total={total} tally={board.tally} marks={marks} tall />

        <div className="breakdown">
          <Share label="За" dot="green" value={board.tally.for} total={total} />
          <Share label="Против" dot="red" value={board.tally.against} total={total} />
          <Share label="Воздержались" dot="grey" value={board.tally.abstain} total={total} />
        </div>

        <div className="divider" />

        <div className="stack">
          <QuorumCheck quorum={board.quorum} total={total} />
          <DecisionCheck decision={board.decision} meeting={meeting} total={total} quorum={board.quorum.passed} />
        </div>
      </div>

      {board.pending_claims > 0 && (
        <button type="button" className="card" style={{ marginTop: 12 }} onClick={() => nav.go({ name: 'claims', id })}>
          <div className="card-top">
            <div className="strong">Заявки на проверку</div>
            <span className="count">{board.pending_claims}</span>
          </div>
          <div className="caption" style={{ marginTop: 4 }}>
            Голоса по ним сохранены, но не считаются, пока вы не сверите собственников с реестром
          </div>
        </button>
      )}

      <MyVoteCard meeting={meeting} />

      {open && meeting.invite_link && (
        <div style={{ marginTop: 12 }}>
          <InviteCard link={meeting.invite_link} question={meeting.question} />
        </div>
      )}

      {notVoted.length > 0 && (
        <>
          <div style={{ height: 20 }} />
          <SectionTitle count={notVoted.length} aside={`${area(board.not_voted_area)} · ${percent(total > 0 ? board.not_voted_area / total : 0)}`}>
            Не проголосовали
          </SectionTitle>
          <div className="list">
            {notVoted.slice(0, shown).map((gap) => {
              const name = (gap.owners ?? []).filter(Boolean).map(shortName).join(', ') || 'Собственник не указан';
              return (
                <div key={gap.number} className="list-row">
                  <div className="inner">
                    <div className="avatar" aria-hidden="true">{initials(gap.owners?.[0] ?? '')}</div>
                    <div className="body">
                      <div className="strong">{name}</div>
                      <div className="caption">кв. {gap.number} · {area(gap.area)}</div>
                    </div>
                  </div>
                </div>
              );
            })}
            {notVoted.length > shown && (
              <button type="button" className="list-more" onClick={() => setShown((n) => n + 20)}>
                Показать ещё {Math.min(20, notVoted.length - shown)}
              </button>
            )}
          </div>
          <p className="caption" style={{ padding: '8px 28px 0' }}>
            Сверху — самые крупные: обход в этом порядке быстрее всего закрывает разрыв.
          </p>
        </>
      )}

      {board.pending_claims > 0 && (
        <BottomBar>
          <button type="button" className="btn primary large" onClick={() => nav.go({ name: 'claims', id })}>
            Проверить заявки · {board.pending_claims}
          </button>
        </BottomBar>
      )}
    </div>
  );
}

// Инициатор — обычно тоже собственник, и голосует тем же путём, что соседи.
function MyVoteCard({ meeting }: { meeting: Meeting }) {
  const nav = useNav();
  const open = meeting.status === 'active';
  const hasFlat = meeting.claims.length > 0;
  if (!open && !meeting.choice) return null;

  const flats = meeting.claims.map((c) => `кв. ${c.flat_number}`).join(', ');

  return (
    <div className="card" style={{ marginTop: 12 }}>
      <div className="card-top">
        <div className="strong">Ваш голос</div>
        {meeting.choice
          ? <Badge tone="green">{choiceLabels[meeting.choice]}</Badge>
          : hasFlat && <Badge tone="blue">Не отдан</Badge>}
      </div>
      <div className="caption" style={{ marginTop: 4 }}>
        {hasFlat
          ? `${flats}${meeting.voted_at ? ` · ${dayTime(meeting.voted_at)}` : ''}`
          : 'Если вы собственник в этом доме, выберите свою квартиру и проголосуйте'}
      </div>
      {open && (
        <button type="button" className={`btn ${meeting.choice ? 'secondary' : 'primary'}`}
          onClick={() => nav.go(hasFlat ? { name: 'vote', id: meeting.id } : { name: 'pick', id: meeting.id })}>
          {!hasFlat ? 'Выбрать квартиру' : meeting.choice ? 'Изменить ответ' : 'Проголосовать'}
        </button>
      )}
    </div>
  );
}

function Share({ label, dot, value, total }: { label: string; dot: string; value: number; total: number }) {
  return (
    <div>
      <div className="caption legend"><span className={`dot ${dot}`} />{label}</div>
      <div className="title" style={{ marginTop: 2 }}>{percent(total > 0 ? value / total : 0)}</div>
      <div className="caption">{area(value)}</div>
    </div>
  );
}

function QuorumCheck({ quorum, total }: { quorum: Threshold; total: number }) {
  const turnout = total > 0 ? quorum.reached / total : 0;
  return (
    <Check passed={quorum.passed} progress={quorum.required > 0 ? quorum.reached / quorum.required : 0}
      title={quorum.passed ? 'Кворум набран' : 'Кворума пока нет'}
      caption={quorum.passed
        ? `Проголосовали ${percent(turnout)} голосов, нужно более 50%`
        : shortfall(quorum, total)} />
  );
}

function DecisionCheck({ decision, meeting, total, quorum }: {
  decision: Threshold; meeting: Meeting; total: number; quorum: boolean;
}) {
  const fromTurnout = meeting.rule.base === 'participating';
  const share = decision.base > 0 ? decision.reached / decision.base : 0;

  let caption: string;
  if (decision.passed) {
    caption = fromTurnout
      ? `«За» ${percent(share)} от проголосовавших, нужно более 50%`
      : `«За» ${percent(share)} голосов дома`;
    if (!quorum) caption += ' — но без кворума решение не принимается';
  } else if (fromTurnout && decision.base === 0) {
    caption = 'Пока никто не проголосовал';
  } else {
    caption = shortfall(decision, fromTurnout ? decision.base : total);
  }

  return (
    <Check passed={decision.passed && quorum} progress={decision.required > 0 ? decision.reached / decision.required : 0}
      title={`${decision.label} ${decision.passed ? 'набрано' : 'пока не набрано'}`}
      caption={caption} />
  );
}

// «Не хватает 539 м², это ещё 11,7% голосов». При «более чем» ровно порога мало.
function shortfall(threshold: Threshold, total: number): string {
  if (threshold.gap <= 0) return 'Не хватает ещё хотя бы одного голоса';
  return `Не хватает ${area(threshold.gap)}, это ещё ${percent(total > 0 ? threshold.gap / total : 0)} голосов`;
}

function Check({ passed, progress, title, caption }: { passed: boolean; progress: number; title: string; caption: string }) {
  return (
    <div className="check">
      {passed ? <CheckCircle /> : <ProgressRing share={progress} />}
      <div className="body">
        <div className="strong">{title}</div>
        <div className="caption">{caption}</div>
      </div>
    </div>
  );
}

