-- Выбор участника хранится отдельно от подтверждения инициатором.
ALTER TABLE claims ADD COLUMN requested_owner_id INTEGER REFERENCES owners(id) ON DELETE SET NULL;
UPDATE claims SET requested_owner_id = owner_id WHERE owner_id IS NOT NULL;
