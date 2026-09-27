-- Пользователи MAX, которые заходили в бота или мини-приложение.
-- Собственник из реестра, никогда не заходивший в MAX, здесь не появляется.
CREATE TABLE users (
    id         SERIAL PRIMARY KEY,
    max_id     BIGINT      NOT NULL UNIQUE,
    full_name  VARCHAR(255),
    username   VARCHAR(255),
    phone      VARCHAR(32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Диалоги с ботом: бот не может написать первым тому,
-- кто сам не начал переписку. Без этой таблицы некуда слать бланки.
CREATE TABLE bot_dialogs (
    max_id     BIGINT PRIMARY KEY,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_active  BOOLEAN     NOT NULL DEFAULT true
);

-- Дом. Ключ — chat_id домового чата: он стабилен, в отличие от адреса.
CREATE TABLE houses (
    id         SERIAL PRIMARY KEY,
    chat_id    BIGINT UNIQUE,
    address    VARCHAR(512) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Собрание. Здесь же лежит снимок исходных данных на дату:
-- общая площадь и правило порога. Закон и площади могут поменяться,
-- результат прошедшего собрания — нет.
CREATE TABLE votings (
    id                SERIAL PRIMARY KEY,
    house_id          INTEGER      NOT NULL REFERENCES houses (id) ON DELETE CASCADE,
    initiator_user_id INTEGER      NOT NULL REFERENCES users (id),
    question          TEXT         NOT NULL,
    category          VARCHAR(32)  NOT NULL DEFAULT 'simple',
    rule_json         JSONB        NOT NULL,
    total_area        NUMERIC(12, 2) NOT NULL CHECK (total_area > 0),
    entrances_count   INTEGER      NOT NULL DEFAULT 1,
    invite_token      VARCHAR(64)  NOT NULL UNIQUE,
    status            VARCHAR(16)  NOT NULL DEFAULT 'draft'
                      CHECK (status IN ('draft', 'active', 'finished')),
    starts_at         TIMESTAMPTZ,
    ends_at           TIMESTAMPTZ,
    notice_sent_at    TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_votings_house ON votings (house_id);
CREATE INDEX idx_votings_active ON votings (status, ends_at);

-- Помещения. Привязаны к собранию, а не к дому: реестр грузится
-- заново на каждое собрание как снимок на дату.
CREATE TABLE flats (
    voting_id INTEGER        NOT NULL REFERENCES votings (id) ON DELETE CASCADE,
    id        SERIAL PRIMARY KEY,
    number    VARCHAR(32)    NOT NULL,
    entrance  INTEGER,
    floor     INTEGER,
    area      NUMERIC(10, 2) NOT NULL CHECK (area > 0),
    UNIQUE (voting_id, number)
);

CREATE INDEX idx_flats_voting ON flats (voting_id);

-- Собственники из реестра. Существуют независимо от MAX:
-- user_id заполняется, только если человек зашёл и заявил помещение.
CREATE TABLE owners (
    id            SERIAL PRIMARY KEY,
    flat_id       INTEGER       NOT NULL REFERENCES flats (id) ON DELETE CASCADE,
    kind          VARCHAR(16)   NOT NULL DEFAULT 'person'
                  CHECK (kind IN ('person', 'org')),
    full_name     VARCHAR(255),
    org_name      VARCHAR(255),
    ogrn          VARCHAR(32),
    share         NUMERIC(10, 6) NOT NULL CHECK (share > 0 AND share <= 1),
    ownership_doc VARCHAR(255),
    user_id       INTEGER       REFERENCES users (id) ON DELETE SET NULL,
    status        VARCHAR(16)   NOT NULL DEFAULT 'unclaimed'
                  CHECK (status IN ('unclaimed', 'pending', 'confirmed', 'rejected')),
    claimed_at    TIMESTAMPTZ,
    confirmed_at  TIMESTAMPTZ
);

CREATE INDEX idx_owners_flat ON owners (flat_id);
CREATE INDEX idx_owners_user ON owners (user_id);

-- Голоса. weight_area фиксируется в момент голосования:
-- правка реестра не меняет задним числом уже подсчитанный результат.
CREATE TABLE votes (
    id          SERIAL PRIMARY KEY,
    voting_id   INTEGER        NOT NULL REFERENCES votings (id) ON DELETE CASCADE,
    owner_id    INTEGER        NOT NULL REFERENCES owners (id) ON DELETE CASCADE,
    value       VARCHAR(16)    NOT NULL
                CHECK (value IN ('for', 'against', 'abstain')),
    weight_area NUMERIC(10, 2) NOT NULL CHECK (weight_area > 0),
    status      VARCHAR(16)    NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'confirmed')),
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT now(),
    -- переголосование до дедлайна идёт через upsert
    UNIQUE (voting_id, owner_id)
);

CREATE INDEX idx_votes_voting ON votes (voting_id);
