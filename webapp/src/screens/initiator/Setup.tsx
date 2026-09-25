import { useRef, useState, type ReactNode } from 'react';

import { api, ApiError } from '../../api/client';
import type { Meeting, RegistryReport } from '../../api/types';
import { DeleteMeeting } from '../../components/DeleteMeeting';
import { BottomBar, CheckCircle, Failure, Loading, SectionTitle, useLoad } from '../../components/ui';
import { haptic } from '../../lib/bridge';
import { area, dayTime, plural } from '../../lib/format';
import { useNav } from '../../lib/nav';

// Черновик собрания: данные, реестр, чат — и публикация.
export function Setup({ id }: { id: number }) {
  const nav = useNav();
  const { data: meeting, setData, error, reload } = useLoad(() => api.meeting(id), [id]);
  const [report, setReport] = useState<RegistryReport | null>(null);
  const [uploading, setUploading] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);

  if (error) return <Failure message={error} onRetry={reload} />;
  if (!meeting) return <Loading />;

  const hasRegistry = (meeting.registry?.flats ?? 0) > 0;

  async function upload(file: File) {
    setUploading(true);
    try {
      const result = await api.uploadRegistry(id, file);
      setReport(result);
      setData(await api.meeting(id));
      haptic.success();
    } catch (err) {
      nav.showError(err instanceof ApiError ? err.message : 'Не удалось загрузить реестр');
    } finally {
      setUploading(false);
      if (fileInput.current) fileInput.current.value = '';
    }
  }

  async function publish() {
    setPublishing(true);
    try {
      await api.publish(id);
      haptic.success();
      nav.reset({ name: 'dashboard', id });
    } catch (err) {
      nav.showError(err instanceof ApiError ? err.message : 'Не удалось опубликовать');
      setPublishing(false);
    }
  }

  return (
    <div className="screen with-bar">
      <div className="header">
        <div className="caption">Черновик · {meeting.address}</div>
        <h1 className="h1">{meeting.question}</h1>
      </div>

      <SectionTitle>Вопрос и срок</SectionTitle>
      <Step done action={<button type="button" className="btn link" style={{ width: 'auto', padding: 0 }}
        onClick={() => nav.go({ name: 'edit', id })}>Изменить</button>}>
        <div className="strong">{meeting.rule.label}</div>
        <div className="caption">До {dayTime(meeting.ends_at)}</div>
      </Step>

      <div className="gap" />
      <SectionTitle>Реестр собственников</SectionTitle>
      <Step done={hasRegistry}>
        {hasRegistry ? (
          <>
            <div className="strong">
              {meeting.registry!.flats} {plural(meeting.registry!.flats, ['помещение', 'помещения', 'помещений'])} · {area(meeting.registry!.area)}
            </div>
            {report && (
              <div className="caption">
                {report.owners} {plural(report.owners, ['собственник', 'собственника', 'собственников'])}
              </div>
            )}
          </>
        ) : (
          <>
            <div className="strong">Загрузите файл CSV</div>
            <div className="caption">
              Столбцы: № помещения, Адрес объекта, Правообладатель, Долевая площадь, Общая площадь.
              Один собственник — одна строка.
            </div>
          </>
        )}
        <input ref={fileInput} type="file" accept=".csv,text/csv" hidden
          onChange={(e) => { const file = e.target.files?.[0]; if (file) void upload(file); }} />
        <button type="button" className={`btn ${hasRegistry ? 'secondary' : 'primary'}`} style={{ marginTop: 12 }}
          disabled={uploading} onClick={() => fileInput.current?.click()}>
          {uploading ? 'Проверяем файл…' : hasRegistry ? 'Загрузить заново' : 'Выбрать файл'}
        </button>
      </Step>

      {report && report.warnings.length > 0 && (
        <div className="banner orange" style={{ marginTop: 12 }}>
          <div className="body">
            <div className="strong">Проверьте реестр</div>
            {report.warnings.slice(0, 5).map((warning) => (
              <div key={warning} className="caption">• {warning}</div>
            ))}
            {report.warnings.length > 5 && <div className="caption">и ещё {report.warnings.length - 5}</div>}
          </div>
        </div>
      )}

      <div className="gap" />
      <SectionTitle>Площадь дома</SectionTitle>
      <AreaStep meeting={meeting} onChanged={setData} />

      <div className="gap" />
      <SectionTitle>Домовой чат</SectionTitle>
      <Step done={meeting.chat_bound}>
        {meeting.chat_bound ? (
          <>
            <div className="strong">Чат привязан</div>
            <div className="caption">После публикации бот отправит туда вопрос с кнопкой «Проголосовать»</div>
          </>
        ) : (
          <>
            <div className="strong">Необязательно</div>
            <div className="caption">
              Добавьте бота в чат дома и выберите это собрание — вопрос опубликуется туда автоматически.
              Без чата разошлите ссылку на бота соседям сами.
            </div>
          </>
        )}
      </Step>

      <div style={{ padding: '0 12px' }}>
        <DeleteMeeting meeting={meeting} />
      </div>

      <BottomBar hint={hasRegistry ? 'После публикации вопрос и реестр изменить нельзя' : 'Сначала загрузите реестр'}>
        <button type="button" className="btn primary large" disabled={!hasRegistry || publishing} onClick={publish}>
          {publishing ? 'Публикуем…' : 'Опубликовать'}
        </button>
      </BottomBar>
    </div>
  );
}

// Площадь дома — база кворума. По умолчанию это сумма помещений из реестра;
// вручную — если по техпаспорту она больше (в реестре не все помещения).
function AreaStep({ meeting, onChanged }: { meeting: Meeting; onChanged: (m: Meeting) => void }) {
  const nav = useNav();
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState('');
  const [saving, setSaving] = useState(false);

  const manual = meeting.total_area_source === 'manual';
  const registryArea = meeting.registry?.area ?? 0;

  async function save(totalArea: number) {
    setSaving(true);
    try {
      onChanged(await api.setTotalArea(meeting.id, totalArea));
      setEditing(false);
      haptic.success();
    } catch (err) {
      nav.showError(err instanceof ApiError ? err.message : 'Не удалось сохранить площадь');
    } finally {
      setSaving(false);
    }
  }

  const typed = Number(value.replace(',', '.').replace(/\s/g, ''));

  return (
    <Step done={meeting.total_area > 0}>
      {meeting.total_area > 0 ? (
        <>
          <div className="strong">{area(meeting.total_area)}</div>
          <div className="caption">
            {manual
              ? `указана вручную${registryArea > 0 ? ` · в реестре ${area(registryArea)}` : ''}`
              : 'сумма площадей помещений из реестра'}
          </div>
        </>
      ) : (
        <>
          <div className="strong">Посчитается из реестра</div>
          <div className="caption">Сумма площадей всех помещений в файле</div>
        </>
      )}
      <div className="caption" style={{ marginTop: 4 }}>От неё считается кворум — нужно больше половины.</div>

      {editing ? (
        <div style={{ marginTop: 12, display: 'flex', flexDirection: 'column', gap: 8 }}>
          <input className="input" inputMode="decimal" autoFocus placeholder="Площадь по техпаспорту, м²"
            value={value} onChange={(e) => setValue(e.target.value)} aria-label="Площадь дома, м²" />
          <div className="btn-row">
            <button type="button" className="btn secondary" disabled={saving} onClick={() => setEditing(false)}>Отмена</button>
            <button type="button" className="btn primary" disabled={saving || !(typed > 0)} onClick={() => save(typed)}>
              {saving ? 'Сохраняем…' : 'Сохранить'}
            </button>
          </div>
        </div>
      ) : (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginTop: 4 }}>
          <button type="button" className="btn link" style={{ width: 'auto', paddingLeft: 0 }}
            onClick={() => { setValue(manual ? String(meeting.total_area).replace('.', ',') : ''); setEditing(true); }}>
            {manual ? 'Изменить' : 'По техпаспорту больше? Указать вручную'}
          </button>
          {manual && (
            <button type="button" className="btn link" style={{ width: 'auto' }} disabled={saving} onClick={() => save(0)}>
              Вернуть по реестру
            </button>
          )}
        </div>
      )}
    </Step>
  );
}

function Step({ done, action, children }: { done: boolean; action?: ReactNode; children: ReactNode }) {
  return (
    <div className="card">
      <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
        {done ? <CheckCircle /> : <span className="radio-placeholder" style={{
          width: 24, height: 24, borderRadius: '50%', border: '2px solid var(--icon-mute)', flexShrink: 0,
        }} />}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flexGrow: 1, minWidth: 0 }}>{children}</div>
        {action}
      </div>
    </div>
  );
}
