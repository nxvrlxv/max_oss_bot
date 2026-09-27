-- Площадь дома по умолчанию — сумма площадей помещений из реестра.
-- До загрузки реестра её нет, поэтому NULL. Ручное значение остаётся
-- как дополнительная возможность: если в техпаспорте площадь больше,
-- чем набралось в реестре, кворум считается от неё.
ALTER TABLE votings ALTER COLUMN total_area DROP NOT NULL;

-- Уже созданные собрания вводили площадь вручную — так и помечаем.
ALTER TABLE votings
    ADD COLUMN total_area_source VARCHAR(16) NOT NULL DEFAULT 'manual'
    CHECK (total_area_source IN ('registry', 'manual'));
