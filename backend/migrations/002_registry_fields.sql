-- Поля, которые приходят из выписки ЕГРН и нужны бланку и подсчёту.

-- Долевая площадь из реестра — точный вес голоса. Пересчёт через
-- share даёт погрешность: 1/3 от 60 м² при шести знаках доли — 19.99998.
ALTER TABLE owners
    ADD COLUMN owned_area NUMERIC(10, 2) CHECK (owned_area > 0);

ALTER TABLE flats
    ADD COLUMN cadastral_number VARCHAR(64),
    ADD COLUMN premises_type    VARCHAR(64);
