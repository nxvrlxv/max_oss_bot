import { initData } from '../lib/bridge';
import type {
  Claim, Dashboard, Flat, Me, Meeting, MeetingInput, PendingClaim, RegistryReport, Choice,
} from './types';

/** Ошибка API: текст уже по-русски, его можно показать как есть. */
export class ApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown, contentType = 'application/json'): Promise<T> {
  const headers: Record<string, string> = { 'X-Max-Init-Data': initData() };
  let payload: BodyInit | undefined;
  if (body instanceof Blob) {
    headers['Content-Type'] = contentType;
    payload = body;
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }

  let response: Response;
  try {
    response = await fetch(`/api${path}`, { method, headers, body: payload });
  } catch {
    throw new ApiError('Нет соединения. Проверьте интернет и попробуйте ещё раз', 0);
  }

  if (response.status === 204) return undefined as T;

  const data = await response.json().catch(() => null);
  if (!response.ok) {
    throw new ApiError(data?.error ?? 'Что-то пошло не так, попробуйте ещё раз', response.status);
  }
  return data as T;
}

export const api = {
  me: () => request<Me>('GET', '/me'),
  meeting: (id: number) => request<Meeting>('GET', `/meetings/${id}`),
  flats: (id: number) => request<Flat[]>('GET', `/meetings/${id}/flats`),
  claim: (id: number, flatNumber: string) =>
    request<Claim>('POST', `/meetings/${id}/claims`, { flat_number: flatNumber }),
  cancelClaim: (id: number, claimId: number) => request<void>('DELETE', `/meetings/${id}/claims/${claimId}`),
  vote: (id: number, choice: Choice) => request<Meeting>('POST', `/meetings/${id}/vote`, { choice }),

  create: (input: MeetingInput) => request<Meeting>('POST', '/meetings', input),
  update: (id: number, input: MeetingInput) => request<Meeting>('PUT', `/meetings/${id}`, input),
  uploadRegistry: (id: number, file: File) =>
    request<RegistryReport>('POST', `/meetings/${id}/registry`, file, 'text/csv'),
  publish: (id: number) => request<Meeting>('POST', `/meetings/${id}/publish`),
  dashboard: (id: number) => request<Dashboard>('GET', `/meetings/${id}/dashboard`),
  pendingClaims: (id: number) => request<PendingClaim[]>('GET', `/meetings/${id}/claims`),
  confirmClaim: (id: number, claimId: number, ownerId: number) =>
    request<void>('POST', `/meetings/${id}/claims/${claimId}/confirm`, { owner_id: ownerId }),
  rejectClaim: (id: number, claimId: number) => request<void>('POST', `/meetings/${id}/claims/${claimId}/reject`),
};
