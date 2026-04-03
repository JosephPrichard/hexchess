-- +goose up
BEGIN;

-- 1. Add temp UUID column
ALTER TABLE replays ADD COLUMN temp_uuid UUID UNIQUE NOT NULL DEFAULT gen_random_uuid();

-- 2. Cast TEXT directly to UUID
UPDATE replays SET temp_uuid = game_id::UUID;

-- 3. Drop old column and rename
ALTER TABLE replays DROP COLUMN game_id;
ALTER TABLE replays RENAME COLUMN temp_uuid TO game_id;

COMMIT;

-- +goose down
BEGIN;

-- 1. Add temp TEXT column
ALTER TABLE replays ADD COLUMN temp_text TEXT UNIQUE NOT NULL DEFAULT gen_random_uuid();

-- 2. Cast UUID back to TEXT
UPDATE replays SET temp_text = game_id::TEXT;

-- 3. Drop old column and rename
ALTER TABLE replays DROP COLUMN game_id;
ALTER TABLE replays RENAME COLUMN temp_text TO game_id;

COMMIT;