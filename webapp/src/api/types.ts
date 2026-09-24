// Типы ответов API — зеркало JSON из backend/internal/api.

export type Choice = 'for' | 'against' | 'abstain';
export type MeetingStatus = 'draft' | 'active' | 'finished';
export type ClaimStatus = 'pending' | 'confirmed' | 'rejected';
export type RuleKind = 'soft' | 'hard' | 'all' | 'custom';

export interface Rule {
  kind: RuleKind;
  label: string;
  base: 'total' | 'participating';
  threshold: number;
  op: '>' | '>=';
}

export interface Claim {
  id: number;
  flat_id: number;
  flat_number: string;
  flat_area: number;
  status: ClaimStatus;
  owner_name?: string; // кем подтверждён по реестру
  weight: number;
  choice?: Choice;
  voted_at?: string;
  created_at: string;
}

export interface Meeting {
  id: number;
  question: string;
  address: string;
  status: MeetingStatus;
  starts_at?: string;
  ends_at?: string;
  rule: Rule;
  total_area: number;
  entrances_count: number;
  is_initiator: boolean;
  chat_bound: boolean;
  invite_link?: string;
  claims: Claim[];
  choice?: Choice;
  voted_at?: string;
  registry?: { flats: number; area: number };
  summary?: { turnout: number; for: number; against: number; abstain: number; quorum: boolean; accepted: boolean };
}

export interface Me {
  user: { id: number; name: string };
  meetings: Meeting[];
}

export interface Flat {
  id: number;
  number: string;
  area: number;
}

export interface Threshold {
  label: string;
  threshold: number;
  strict: boolean;
  base: number;
  reached: number;
  required: number;
  gap: number;
  passed: boolean;
}

export interface FlatGap {
  number: string;
  area: number;
  owners: string[] | null;
}

export interface Dashboard {
  total_area: number;
  tally: { for: number; against: number; abstain: number; total: number };
  quorum: Threshold;
  decision: Threshold;
  accepted: boolean;
  flats_total: number;
  flats_voted: number;
  not_voted: FlatGap[] | null;
  not_voted_area: number;
  pending_claims: number;
  pending_votes: { count: number; area: number };
}

export interface RegistryOwner {
  id: number;
  name: string;
  owned_area: number;
  taken: boolean;
}

export interface PendingClaim extends Claim {
  user_name: string;
  confirmed_as?: string; // уже подтверждён под этим ФИО по другой квартире
  owners: RegistryOwner[] | null;
}

export interface RegistryReport {
  house_address: string;
  flats: number;
  owners: number;
  flats_area: string;
  owned_area: string;
  warnings: string[];
}

export interface MeetingInput {
  address: string;
  question: string;
  rule: 'soft' | 'hard' | 'all';
  total_area: number;
  entrances_count: number;
  ends_at: string;
}

export const choiceLabels: Record<Choice, string> = {
  for: 'За',
  against: 'Против',
  abstain: 'Воздержался',
};
