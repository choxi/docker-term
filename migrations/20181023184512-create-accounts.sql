-- +migrate Up

CREATE TABLE accounts (
    id SERIAL PRIMARY KEY,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp
);

CREATE TRIGGER set_accounts_timestamps 
BEFORE UPDATE ON accounts FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

ALTER TABLE images DROP COLUMN user_id;
ALTER TABLE images ADD COLUMN account_id integer;
CREATE INDEX idx_images_on_account_id ON images (account_id);

ALTER TABLE users ADD COLUMN account_id integer;
CREATE INDEX idx_users_on_account_id ON users (account_id);


-- +migrate Down

DROP INDEX idx_users_on_account_id;
ALTER TABLE users DROP COLUMN account_id;

DROP INDEX idx_images_on_account_id;
ALTER TABLE images DROP COLUMN account_id;
ALTER TABLE images ADD COLUMN user_id integer;

DROP TRIGGER set_users_timestamps ON users;

DROP TABLE accounts;
