-- Sian Game database schema (PostgreSQL 14+)
BEGIN;

CREATE TABLE genres (
    id    SERIAL       PRIMARY KEY,
    name  VARCHAR(50)  NOT NULL UNIQUE
);

CREATE TABLE games (
    id              BIGSERIAL      PRIMARY KEY,
    title           VARCHAR(255)   NOT NULL CHECK (char_length(btrim(title)) >= 1),
    description_th  TEXT           NOT NULL,
    developer       VARCHAR(255)   NOT NULL,
    release_date    DATE,
    price           NUMERIC(10,2)  NOT NULL CHECK (price > 0),
    cover_url       VARCHAR(1000)  NOT NULL,
    is_active       BOOLEAN        NOT NULL DEFAULT TRUE,   -- FALSE = เลิกขาย แต่ยังอยู่ในคลังของคนที่ซื้อแล้ว
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ    NOT NULL DEFAULT now()
);

CREATE TABLE game_genres (
    game_id   BIGINT   NOT NULL REFERENCES games (id) ON DELETE CASCADE,
    genre_id  INTEGER  NOT NULL REFERENCES genres (id),
    PRIMARY KEY (game_id, genre_id)
);

CREATE TABLE game_screenshots (
    game_id   BIGINT         NOT NULL REFERENCES games (id) ON DELETE CASCADE,
    url       VARCHAR(1000)  NOT NULL,
    position  SMALLINT       NOT NULL,
    PRIMARY KEY (game_id, position)
);

CREATE TABLE members (
    id             BIGSERIAL     PRIMARY KEY,
    email          VARCHAR(255)  NOT NULL,
    password_hash  VARCHAR(255)  NOT NULL,   -- bcrypt ห้ามเก็บรหัสผ่านตรง ๆ
    display_name   VARCHAR(50)   NOT NULL CHECK (char_length(btrim(display_name)) >= 1),
    role           VARCHAR(10)   NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_members_email ON members (lower(email));

CREATE TABLE cart_items (
    member_id  BIGINT       NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    game_id    BIGINT       NOT NULL REFERENCES games (id),
    added_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (member_id, game_id)
);

CREATE TABLE orders (
    id          BIGSERIAL      PRIMARY KEY,
    member_id   BIGINT         NOT NULL REFERENCES members (id),
    total       NUMERIC(10,2)  NOT NULL CHECK (total >= 0),
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT now()
);
CREATE INDEX idx_orders_member ON orders (member_id, created_at DESC);

CREATE TABLE order_items (
    order_id  BIGINT         NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    game_id   BIGINT         NOT NULL REFERENCES games (id),
    price     NUMERIC(10,2)  NOT NULL CHECK (price >= 0),   -- ราคา ณ วันที่ซื้อ
    PRIMARY KEY (order_id, game_id)
);

CREATE TABLE library_items (
    member_id    BIGINT       NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    game_id      BIGINT       NOT NULL REFERENCES games (id),
    order_id     BIGINT       NOT NULL REFERENCES orders (id),
    acquired_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (member_id, game_id)                        -- กันซื้อเกมเดิมซ้ำ
);

CREATE TABLE wishlist_items (
    member_id  BIGINT       NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    game_id    BIGINT       NOT NULL REFERENCES games (id),
    added_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (member_id, game_id)
);

CREATE TABLE reviews (
    id          BIGSERIAL    PRIMARY KEY,
    member_id   BIGINT       NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    game_id     BIGINT       NOT NULL REFERENCES games (id),
    rating      SMALLINT     NOT NULL CHECK (rating BETWEEN 1 AND 5),
    text        TEXT         NOT NULL CHECK (char_length(btrim(text)) >= 1),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (member_id, game_id)
);
CREATE INDEX idx_reviews_game ON reviews (game_id, created_at DESC);

-- ปิดการเข้าถึงผ่าน Data API ของ Supabase ไม่ต้องมี policy เพราะ Go ต่อด้วยเจ้าของตาราง
ALTER TABLE genres           ENABLE ROW LEVEL SECURITY;
ALTER TABLE games            ENABLE ROW LEVEL SECURITY;
ALTER TABLE game_genres      ENABLE ROW LEVEL SECURITY;
ALTER TABLE game_screenshots ENABLE ROW LEVEL SECURITY;
ALTER TABLE members          ENABLE ROW LEVEL SECURITY;
ALTER TABLE cart_items       ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders           ENABLE ROW LEVEL SECURITY;
ALTER TABLE order_items      ENABLE ROW LEVEL SECURITY;
ALTER TABLE library_items    ENABLE ROW LEVEL SECURITY;
ALTER TABLE wishlist_items   ENABLE ROW LEVEL SECURITY;
ALTER TABLE reviews          ENABLE ROW LEVEL SECURITY;

COMMIT;
