DROP DATABASE IF EXISTS isuumo;
CREATE DATABASE isuumo;

DROP TABLE IF EXISTS isuumo.estate;
DROP TABLE IF EXISTS isuumo.chair;

CREATE TABLE isuumo.estate
(
    id          INTEGER             NOT NULL PRIMARY KEY,
    name        VARCHAR(64)         NOT NULL,
    description VARCHAR(4096)       NOT NULL,
    thumbnail   VARCHAR(128)        NOT NULL,
    address     VARCHAR(128)        NOT NULL,
    latitude    DOUBLE PRECISION    NOT NULL,
    longitude   DOUBLE PRECISION    NOT NULL,
    rent        INTEGER             NOT NULL,
    door_height INTEGER             NOT NULL,
    door_width  INTEGER             NOT NULL,
    features    VARCHAR(64)         NOT NULL,
    popularity  INTEGER             NOT NULL
);

CREATE TABLE isuumo.chair
(
    id          INTEGER         NOT NULL PRIMARY KEY,
    name        VARCHAR(64)     NOT NULL,
    description VARCHAR(4096)   NOT NULL,
    thumbnail   VARCHAR(128)    NOT NULL,
    price       INTEGER         NOT NULL,
    height      INTEGER         NOT NULL,
    width       INTEGER         NOT NULL,
    depth       INTEGER         NOT NULL,
    color       VARCHAR(64)     NOT NULL,
    features    VARCHAR(64)     NOT NULL,
    kind        VARCHAR(64)     NOT NULL,
    popularity  INTEGER         NOT NULL,
    stock       INTEGER         NOT NULL
);

ALTER TABLE isuumo.estate ADD COLUMN `popularity_m` INTEGER AS (-`popularity`) STORED;
ALTER TABLE isuumo.estate ADD KEY `popularity_m_id` (`popularity_m`, `id`);
-- explain SELECT * FROM estate WHERE rent >= 100000 AND rent < 150000 ORDER BY popularity_m ASC, id ASC LIMIT 25 OFFSET 75;

ALTER TABLE isuumo.estate ADD KEY `rent_id` (`rent`, `id`);
-- EXPLAIN SELECT COUNT(*) FROM estate WHERE rent >= 100000 AND rent < 150000;
-- EXPLAIN SELECT * FROM estate ORDER BY rent ASC, id ASC LIMIT 20;

ALTER TABLE isuumo.chair ADD INDEX `door_height` (`door_height`);
--  EXPLAIN SELECT COUNT(*) FROM estate WHERE door_height >= 110 AND door_height < 150;
ALTER TABLE isuumo.chair ADD INDEX `door_width` (`door_width`);
--  EXPLAIN SELECT COUNT(*) FROM estate WHERE door_width >= 80 AND door_width < 110\G;

ALTER TABLE isuumo.chair ADD COLUMN `popularity_m` INTEGER AS (-`popularity`) STORED;
ALTER TABLE isuumo.chair ADD COLUMN `in_stock` BOOLEAN AS (`stock` != 0) STORED;

ALTER TABLE isuumo.chair ADD KEY `in_stock_height` (`in_stock`, `height`);
-- EXPLAIN SELECT COUNT(*) FROM chair WHERE height >= 80 AND height < 110 AND `in_stock` = 1;

ALTER TABLE isuumo.chair ADD KEY `in_stock_kind` (`in_stock`, `kind`);
-- EXPLAIN SELECT COUNT(*) FROM chair WHERE kind = 'ゲーミングチェア' AND `in_stock` = 1;

ALTER TABLE isuumo.chair ADD KEY `in_stock_price_id` (`in_stock`, `price`, `id`);
-- EXPLAIN SELECT * FROM chair WHERE `in_stock` = 1 ORDER BY price, id LIMIT 20;

ALTER TABLE isuumo.chair ADD KEY `in_stock_popularity_m_id` (`in_stock`, `popularity_m`, `id`);
-- EXPLAIN SELECT * FROM chair WHERE width >= 110 AND width < 150 AND `in_stock` = 1 ORDER BY popularity_m ASC, id ASC LIMIT 25 OFFSET 0;

ALTER TABLE isuumo.chair ADD INDEX `color_in_stock_popularity_m_id` (`color`, `in_stock`, `popularity_m`, `id`);
--  explain SELECT * FROM chair WHERE color = 'ネイビー' AND `in_stock` = 1 ORDER BY popularity DESC, id ASC LIMIT 25 OFFSET 0;

ALTER TABLE isuumo.chair ADD INDEX `in_stock_width` (`in_stock`, `width`);
--  EXPLAIN SELECT COUNT(*) FROM chair WHERE width >= 80 AND width < 110 AND `in_stock` = 1;
ALTER TABLE isuumo.chair ADD INDEX `in_stock_depth` (`in_stock`, `depth`);
--  EXPLAIN SELECT COUNT(*) FROM chair WHERE depth >= 110 AND depth < 150 AND `in_stock` = 1;

