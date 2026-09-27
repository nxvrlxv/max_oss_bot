-- Чат принадлежит выбранному собранию, а не записи адреса дома.
-- Сохраняем существующие привязки, в том числе у общих записей houses.
ALTER TABLE votings ADD COLUMN chat_id BIGINT;

UPDATE votings v SET chat_id = h.chat_id
FROM houses h WHERE h.id = v.house_id;

CREATE INDEX idx_votings_chat ON votings (chat_id) WHERE chat_id IS NOT NULL;
