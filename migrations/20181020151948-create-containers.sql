-- +migrate Up
-- https://stackoverflow.com/questions/9556474/how-do-i-automatically-update-a-timestamp-in-postgresql

-- +migrate StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()   
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;   
END;
$$ language 'plpgsql';
-- +migrate StatementEnd

CREATE TABLE images (
    id SERIAL,
    uuid varchar,
    source_url varchar,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp
);

CREATE TABLE containers (
    id SERIAL,
    uuid varchar,
    image_id integer,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp
);

CREATE TRIGGER set_images_timestamps 
BEFORE UPDATE ON images FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER set_containers_timestamps
BEFORE UPDATE ON containers FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE UNIQUE INDEX idx_images_on_uuid ON images (uuid);
CREATE UNIQUE INDEX idx_containers_on_uuid ON containers (uuid);
CREATE UNIQUE INDEX idx_containers_on_image_id ON containers (image_id);

-- +migrate Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP INDEX idx_containers_on_image_id;
DROP INDEX idx_containers_on_uuid;
DROP INDEX idx_images_on_uuid;

DROP TRIGGER set_containers_timestamps ON containers;
DROP TRIGGER set_images_timestamps ON images;

DROP TABLE containers;
DROP TABLE images;