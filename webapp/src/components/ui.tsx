import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';

import { ApiError } from '../api/client';

/** Загрузка данных экрана: состояние, ошибка и перезапрос. */
export function useLoad<T>(load: () => Promise<T>, deps: unknown[]) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const loadRef = useRef(load);
  loadRef.current = load;

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setData(await loadRef.current());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить данные');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void reload(); }, deps);

  return { data, setData, error, loading, reload };
}

export function Loading() {
  return (
    <div className="center" aria-busy="true">
      <div className="spinner" />
    </div>
  );
}

export function Failure({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="center">
      <p className="title">{message}</p>
      {onRetry && <button type="button" className="btn secondary" style={{ maxWidth: 240 }} onClick={onRetry}>Повторить</button>}
    </div>
  );
}

export function SectionTitle({ children, count, aside }: { children: ReactNode; count?: number; aside?: ReactNode }) {
  return (
    <div className="section">
      <div className="name">
        {children}
        {count !== undefined && <span className="count">{count}</span>}
      </div>
      {aside && <div className="aside">{aside}</div>}
    </div>
  );
}

type Tone = 'blue' | 'green' | 'red' | 'orange' | 'grey';

export function Badge({ tone = 'grey', dot = tone !== 'grey', tall, children }: {
  tone?: Tone; dot?: boolean; tall?: boolean; children: ReactNode;
}) {
  return (
    <span className={`badge ${tone} ${tall ? 'tall' : ''}`}>
      {dot && <span className={`dot ${tone}`} />}
      {children}
    </span>
  );
}

export function BottomBar({ children, hint }: { children: ReactNode; hint?: ReactNode }) {
  return (
    <div className="bottom-bar mx-up">
      {hint && <p className="caption hint">{hint}</p>}
      <div>{children}</div>
    </div>
  );
}

export function Chevron() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
      strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" style={{ flexShrink: 0, color: 'var(--icon-mute)' }}>
      <path d="M9 18l6-6-6-6" />
    </svg>
  );
}

export function CheckCircle({ size = 24 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" style={{ flexShrink: 0 }}>
      <circle cx="12" cy="12" r="12" fill="#2bc644" />
      <path d="M7 12.5l3.2 3.2L17 8.9" fill="none" stroke="#fff" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

/** Кольцо прогресса: доля от 0 до 1. */
export function ProgressRing({ share }: { share: number }) {
  const length = 2 * Math.PI * 10;
  const filled = Math.max(0, Math.min(1, share)) * length;
  return (
    <svg width="24" height="24" viewBox="0 0 24 24" aria-hidden="true" style={{ flexShrink: 0 }}>
      <circle cx="12" cy="12" r="10" fill="none" stroke="var(--controls-inactive)" strokeWidth="3" />
      <circle cx="12" cy="12" r="10" fill="none" stroke="var(--button-primary)" strokeWidth="3" strokeLinecap="round"
        strokeDasharray={`${filled} ${length}`} transform="rotate(-90 12 12)" />
    </svg>
  );
}

export function InfoIcon({ color = 'var(--button-primary)' }: { color?: string }) {
  return (
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke={color} strokeWidth="2"
      strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" style={{ flexShrink: 0 }}>
      <circle cx="12" cy="12" r="10" />
      <path d="M12 16v-4M12 8h.01" />
    </svg>
  );
}

/** Шкала голосов: сегменты за/против/воздержались и отметки порогов. */
export function VoteScale({ total, tally, marks, tall }: {
  total: number;
  tally: { for: number; against: number; abstain: number };
  marks: { at: number; label?: string; caption?: string; side?: 'left' | 'right' }[];
  tall?: boolean;
}) {
  const width = (value: number) => `${total > 0 ? Math.min(100, (value / total) * 100) : 0}%`;
  const withLabels = marks.some((mark) => mark.label);
  const trackTop = tall ? 8 : 6;
  const trackHeight = tall ? 12 : 8;
  const markHeight = tall ? 28 : 20;

  return (
    <div className="scale" style={{ height: withLabels ? 68 : markHeight, marginTop: tall ? 16 : 12 }}>
      <div className="track" style={{ top: trackTop, height: trackHeight }}>
        <div className="bg-for" style={{ width: width(tally.for) }} />
        <div className="bg-against" style={{ width: width(tally.against) }} />
        <div className="bg-abstain" style={{ width: width(tally.abstain) }} />
      </div>
      {marks.map((mark) => (
        <div key={mark.at} className="mark" style={{ left: `calc(${mark.at * 100}% - 1px)`, height: markHeight }} />
      ))}
      {marks.filter((mark) => mark.label).map((mark) => (
        <div key={`label-${mark.at}`} className={`mark-label ${mark.side === 'left' ? 'left' : ''}`}
          style={mark.side === 'left'
            ? { right: `calc(${(1 - mark.at) * 100}% + 8px)` }
            : { left: `calc(${mark.at * 100}% + 8px)` }}>
          <div className="caption-strong">{mark.label}</div>
          <div className="caption">{mark.caption}</div>
        </div>
      ))}
    </div>
  );
}
