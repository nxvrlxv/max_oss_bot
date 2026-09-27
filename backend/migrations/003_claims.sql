-- Заявки на помещение. Человек выбирает квартиру, а не собственника:
-- ФИО из реестра ему не показываем. Инициатор сверяет заявку с реестром
-- и при подтверждении указывает, какой это собственник.
CREATE TABLE claims (
    id         SERIAL PRIMARY KEY,
    voting_id  INTEGER     NOT NULL REFERENCES votings (id) ON DELETE CASCADE,
    flat_id    INTEGER     NOT NULL REFERENCES flats (id) ON DELETE CASCADE,
    user_id    INTEGER     NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- заполняется при подтверждении: чья доля голосует
    owner_id   INTEGER     REFERENCES owners (id) ON DELETE SET NULL,
    status     VARCHAR(16) NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending', 'confirmed', 'rejected')),
    -- Голос до подтверждения живёт здесь: собственника ещё не знаем,
    -- а значит, и веса. При подтверждении переносится в votes.
    choice     VARCHAR(16) CHECK (choice IN ('for', 'against', 'abstain')),
    voted_at   TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at TIMESTAMPTZ,
    UNIQUE (voting_id, flat_id, user_id)
);

CREATE INDEX idx_claims_voting ON claims (voting_id, status);
CREATE INDEX idx_claims_user ON claims (user_id);

-- Один собственник — одна подтверждённая заявка.
CREATE UNIQUE INDEX idx_claims_owner ON claims (owner_id) WHERE status = 'confirmed';
