import { useEffect, useState, type FormEvent } from 'react';

import { api, ApiError } from '../../api/client';
import type { MeetingInput } from '../../api/types';
import { BottomBar, Failure, Loading } from '../../components/ui';
import { fromMoscowInput, toMoscowInput } from '../../lib/format';
import { useNav } from '../../lib/nav';

type RuleKind = MeetingInput['rule'];

const rules: { kind: RuleKind; title: string; description: string }[] = [
  {
    kind: 'soft',
    title: 'Большинство от проголосовавших',
    description: 'Текущий ремонт, выбор председателя, большинство бытовых вопросов',
  },
  {
    kind: 'hard',
    title: '2/3 от всех собственников',
    description: 'Капремонт, шлагбаум, аренда общего имущества, использование земли',
  },
  {
    kind: 'all',
    title: 'Все собственники',
    description: 'Уменьшение общего имущества, в том числе реконструкция',
  },
];

// Через две недели в 20:00 по Москве — типичный срок заочного голосования.
function defaultDeadline(): string {
  const date = new Date(Date.now() + 14 * 24 * 60 * 60 * 1000);
  return `${toMoscowInput(date.toISOString()).slice(0, 10)}T20:00`;
}

// Создание и правка черновика собрания.
export function MeetingForm({ id }: { id?: number }) {
  const nav = useNav();
  const [loading, setLoading] = useState(Boolean(id));
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [address, setAddress] = useState('');
  const [entrances, setEntrances] = useState('1');
  const [question, setQuestion] = useState('');
  const [rule, setRule] = useState<RuleKind>('soft');
  const [deadline, setDeadline] = useState(defaultDeadline);

  useEffect(() => {
    if (!id) return;
    api.meeting(id)
      .then((m) => {
        setAddress(m.address);
        setEntrances(String(m.entrances_count));
        setQuestion(m.question);
        if (m.rule.kind !== 'custom') setRule(m.rule.kind);
        if (m.ends_at) setDeadline(toMoscowInput(m.ends_at));
      })
      .catch((err) => setLoadError(err instanceof ApiError ? err.message : 'Не удалось загрузить собрание'))
      .finally(() => setLoading(false));
  }, [id]);

  if (loadError) return <Failure message={loadError} />;
  if (loading) return <Loading />;

  // Площадь дома здесь не спрашиваем: она посчитается из реестра на следующем шаге.
  const valid = address.trim() !== '' && question.trim() !== '' && deadline !== '';

  async function submit(event?: FormEvent) {
    event?.preventDefault();
    if (!valid) return;
    setSaving(true);
    const input: MeetingInput = {
      address: address.trim(),
      question: question.trim(),
      rule,
      entrances_count: Math.max(1, Number(entrances) || 1),
      ends_at: fromMoscowInput(deadline),
    };
    try {
      const meeting = id ? await api.update(id, input) : await api.create(input);
      if (id) nav.back();
      else nav.replace({ name: 'setup', id: meeting.id });
    } catch (err) {
      nav.showError(err instanceof ApiError ? err.message : 'Не удалось сохранить');
      setSaving(false);
    }
  }

  return (
    <form className="screen with-bar" onSubmit={submit}>
      <div className="header">
        <div className="caption">{id ? 'Черновик собрания' : 'Шаг 1 из 3'}</div>
        <h1 className="h1">{id ? 'Данные собрания' : 'Новое собрание'}</h1>
      </div>

      <div className="form">
        <label className="field">
          <span className="caption label">Адрес дома</span>
          <input className="input" value={address} onChange={(e) => setAddress(e.target.value)}
            placeholder="ул. Садовая, д. 12" autoComplete="street-address" />
        </label>

        <label className="field">
          <span className="caption label">Подъездов</span>
          <input className="input" value={entrances} onChange={(e) => setEntrances(e.target.value)}
            inputMode="numeric" />
        </label>

        <label className="field">
          <span className="caption label">Вопрос</span>
          <textarea className="input" value={question} onChange={(e) => setQuestion(e.target.value)}
            placeholder="Установить шлагбаум на въезде во двор" maxLength={1000} />
        </label>

        <div className="field">
          <span className="caption label">Как принимается решение</span>
          <div className="stack" style={{ gap: 8 }}>
            {rules.map((option) => (
              <button key={option.kind} type="button" className="option" aria-pressed={rule === option.kind}
                onClick={() => setRule(option.kind)}>
                <span className="radio" />
                <span className="desc">
                  <span className="strong">{option.title}</span>
                  <span className="caption">{option.description}</span>
                </span>
              </button>
            ))}
          </div>
        </div>

        <label className="field">
          <span className="caption label">Голосование до (время московское)</span>
          <input className="input" type="datetime-local" value={deadline} onChange={(e) => setDeadline(e.target.value)} />
        </label>
      </div>

      <BottomBar>
        <button type="submit" className="btn primary large" disabled={!valid || saving}>
          {saving ? 'Сохраняем…' : id ? 'Сохранить' : 'Далее'}
        </button>
      </BottomBar>
    </form>
  );
}
