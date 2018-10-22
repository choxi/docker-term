-- +migrate Up

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username varchar,
    password varchar,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp
);

CREATE TRIGGER set_users_timestamps 
BEFORE UPDATE ON users FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE UNIQUE INDEX idx_users_on_username ON users (username);

-- +migrate Down

DROP INDEX idx_users_on_username;

DROP TRIGGER set_users_timestamps ON users;

DROP TABLE users;

