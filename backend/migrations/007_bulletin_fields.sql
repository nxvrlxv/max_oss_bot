-- Реквизиты права из выписки ЕГРН — для таблицы в бюллетене.
-- ownership_doc остаётся номером регистрации права, дата — отдельно:
-- в бюллетене это разные колонки.
ALTER TABLE owners
    ADD COLUMN registration_date VARCHAR(32),
    ADD COLUMN ownership_type    VARCHAR(128),
    ADD COLUMN share_text        VARCHAR(32);

-- Раньше номер и дата хранились одной строкой «номер от дата».
UPDATE owners
SET registration_date = substring(ownership_doc from ' от (.+)$'),
    ownership_doc     = regexp_replace(ownership_doc, ' от .+$', '')
WHERE ownership_doc LIKE '% от %';
