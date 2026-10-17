BEGIN;

-- Вход ищет пользователя по email в нижнем регистре, поэтому хранится только нормализованная форма.
ALTER TABLE users ADD CONSTRAINT users_email_normalized
 CHECK (email = lower(btrim(email)) AND email ~ '^[^@[:space:]]+@[^@[:space:]]+$');

COMMIT;
