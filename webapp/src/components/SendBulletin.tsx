import { useState } from 'react';

import { ApiError } from '../api/client';
import type { Delivery } from '../api/types';
import { haptic, openBotChat } from '../lib/bridge';
import { useNav } from '../lib/nav';

// Бюллетени приходят PDF-файлом в чат с ботом: в них ФИО и реквизиты права,
// поэтому только в личку. Если бот ещё не может написать первым, открываем
// чат с ним — после «Начать» файл придёт сам.
export function SendBulletin({ label, send }: { label: string; send: () => Promise<Delivery> }) {
  const nav = useNav();
  const [busy, setBusy] = useState(false);

  async function run() {
    setBusy(true);
    try {
      const result = await send();
      if (result.sent) {
        haptic.success();
        nav.showNotice('Файл отправлен в чат с ботом');
      } else if (result.link) {
        openBotChat(result.link);
        nav.showNotice('Нажмите «Начать» в чате с ботом — он пришлёт файл');
      }
    } catch (err) {
      nav.showError(err instanceof ApiError ? err.message : 'Не удалось подготовить файл');
    } finally {
      setBusy(false);
    }
  }

  return (
    <button type="button" className="btn secondary" disabled={busy} onClick={run}>
      {busy ? 'Готовим файл…' : label}
    </button>
  );
}
