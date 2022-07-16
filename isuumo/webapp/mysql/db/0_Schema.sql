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


ALTER TABLE isuumo.chair ADD COLUMN `popularity_m` INTEGER AS (-`popularity`) STORED;
ALTER TABLE isuumo.chair ADD COLUMN `in_stock` BOOLEAN AS (`stock` != 0) STORED;
ALTER TABLE isuumo.chair ADD KEY `in_stock_price_id` (`in_stock`, `price`, `id`);
-- EXPLAIN SELECT * FROM chair WHERE `in_stock` = 1 ORDER BY price, id LIMIT 20;

ALTER TABLE isuumo.chair ADD INDEX `color_in_stock_popularity_m_id` (`color`, `in_stock`, `popularity_m`, `id`);
--  explain SELECT * FROM chair WHERE color = 'ネイビー' AND `in_stock` = 1 ORDER BY popularity DESC, id ASC LIMIT 25 OFFSET 0;

