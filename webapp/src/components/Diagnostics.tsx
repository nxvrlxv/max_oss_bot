import { useEffect, useState } from 'react';

import { copyText, launchInfo } from '../lib/bridge';

// Что приложение получило от MAX при запуске. Внутри MAX нет консоли
// разработчика, поэтому смотрим здесь. Открывается пятью нажатиями
// на заголовок главного экрана. Подписи initData в выводе нет.
export function Diagnostics({ events, onClose }: { events: string[]; onClose: () => void }) {
  const [info, setInfo] = useState<Record<string, string> | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => { void launchInfo().then(setInfo); }, []);

  const text = info
    ? [...Object.entries(info).map(([key, value]) => `${key}: ${value}`), '', 'События:', ...events].join('\n')
    : '';

  return (
    <div className="thanks" role="dialog" aria-label="Диагностика запуска">
      <div style={{ flexGrow: 1, overflow: 'auto', padding: '24px 16px' }}>
        <h1 className="h1">Диагностика</h1>
        <p className="caption" style={{ margin: '4px 0 16px' }}>Скопируйте и пришлите разработчику</p>
        {!info ? <div className="spinner" /> : (
          <div className="list" style={{ margin: 0 }}>
            {Object.entries(info).map(([key, value]) => (
              <div key={key} className="list-row">
                <div className="inner">
                  <div className="body">
                    <div className="caption">{key}</div>
                    <div className="strong" style={{ wordBreak: 'break-all' }}>{value}</div>
                  </div>
                </div>
              </div>
            ))}
            <div className="list-row">
              <div className="inner">
                <div className="body">
                  <div className="caption">События</div>
                  {events.length === 0
                    ? <div className="strong">—</div>
                    : events.map((event, i) => <div key={i} className="caption" style={{ color: 'var(--text-primary)' }}>{event}</div>)}
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
      <div className="actions btn-row">
        <button type="button" className="btn secondary large" onClick={onClose}>Закрыть</button>
        <button type="button" className="btn primary large" disabled={!info}
          onClick={async () => setCopied((await copyText(text)) === 'copied')}>
          {copied ? 'Скопировано' : 'Скопировать'}
        </button>
      </div>
    </div>
  );
}
