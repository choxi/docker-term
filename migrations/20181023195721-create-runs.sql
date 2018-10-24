-- +migrate Up

CREATE TABLE runs (
    id SERIAL PRIMARY KEY,
    container_id integer NOT NULL,
    created_at timestamp default current_timestamp,
    started_at timestamp,
    ended_at timestamp,
    updated_at timestamp default current_timestamp
);

CREATE TRIGGER set_runs_timestamps
BEFORE UPDATE ON runs FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE INDEX idx_runs_on_container_id ON runs (container_id);

-- +migrate Down

DROP INDEX idx_runs_on_container_id;

DROP TRIGGER set_runs_timestamps ON runs;

DROP TABLE runs;