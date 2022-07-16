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

ALTER TABLE isuumo.estate ADD COLUMN `rent_range` INTEGER AS (CASE
    WHEN                    rent <  50000 THEN 0
    WHEN  50000 <= rent AND rent < 100000 THEN 1
    WHEN 100000 <= rent AND rent < 150000 THEN 2
    WHEN 150000 <= rent THEN 3
END) STORED;
ALTER TABLE isuumo.estate ADD COLUMN `door_width_range` INTEGER AS (CASE
    WHEN                    door_width <  80 THEN 0
    WHEN  80 <= door_width AND door_width < 110 THEN 1
    WHEN 110 <= door_width AND door_width < 150 THEN 2
    WHEN 150 <= door_width THEN 3
END) STORED;
ALTER TABLE isuumo.estate ADD COLUMN `door_height_range` INTEGER AS (CASE
    WHEN                    door_height <  80 THEN 0
    WHEN  80 <= door_height AND door_height < 110 THEN 1
    WHEN 110 <= door_height AND door_height < 150 THEN 2
    WHEN 150 <= door_height THEN 3
END) STORED;
ALTER TABLE isuumo.estate ADD COLUMN `popularity_m` INTEGER AS (-`popularity`) STORED;
ALTER TABLE isuumo.estate ADD KEY `popularity_m_id` (`popularity_m`, `id`);
ALTER TABLE isuumo.estate ADD KEY `rent_range_popularity_m_id` (`rent_range`, `popularity_m`, `id`);
-- explain SELECT * FROM estate WHERE rent >= 100000 AND rent < 150000 ORDER BY popularity_m ASC, id ASC LIMIT 25 OFFSET 75;

ALTER TABLE isuumo.estate ADD KEY `rent_range` (`rent_range`);
-- EXPLAIN SELECT COUNT(*) FROM estate WHERE rent >= 100000 AND rent < 150000;

ALTER TABLE isuumo.estate ADD KEY `rent_id` (`rent`, `id`);
-- EXPLAIN SELECT * FROM estate ORDER BY rent ASC, id ASC LIMIT 20;

ALTER TABLE isuumo.estate ADD INDEX `door_height` (`door_height`);
--  EXPLAIN SELECT COUNT(*) FROM estate WHERE door_height >= 110 AND door_height < 150;
ALTER TABLE isuumo.estate ADD INDEX `door_width` (`door_width`);
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




ALTER TABLE isuumo.estate ADD COLUMN `point` GEOMETRY GENERATED ALWAYS AS (Point(latitude, longitude)) STORED;
-- Bug of MySQL 5.7.20 https://bugs.mysql.com/bug.php?id=88972
ALTER TABLE isuumo.estate MODIFY COLUMN `point` GEOMETRY AS (POINT(latitude, longitude)) STORED NOT NULL;
ALTER TABLE isuumo.estate ADD SPATIAL INDEX `point` (`point`);
-- EXPLAIN SELECT * FROM estate WHERE ST_Contains(ST_PolygonFromText('POLYGON((-71.1776585052917 42.3902909739571,-71.1776820268866 42.3903701743239,
-- -71.1776063012595 42.3903825660754,-71.1775826583081 42.3903033653531,-71.1776585052917 42.3902909739571))'), point) ORDER BY popularity_m ASC, id ASC LIMIT 50;
