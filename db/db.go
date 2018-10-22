package db

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	uuid "github.com/satori/go.uuid"
)

type Image struct {
	ID        int    `db:"id" json:"id"`
	UUID      string `db:"uuid" json:"uuid"`
	SourceURL string `db:"source_url" json:"source_url"`
	UpdatedAt string `db:"updated_at" json:"updated_at"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

type Container struct {
	ID        int    `db:"id" json:"id"`
	UUID      string `db:"uuid" json:"uuid"`
	ImageID   int    `db:"image_id" json:"image_id"`
	UpdatedAt string `db:"updated_at" json:"updated_at"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

type DB struct {
	connection *sqlx.DB
}

type queryParams map[string]interface{}

func (d *DB) Connection() *sqlx.DB {
	return d.connection
}

func Connect() DB {
	var (
		dbname     string
		params     string
		connection *sqlx.DB
		err        error
	)

	dbname = "dre_development"
	params = fmt.Sprintf("dbname=%s sslmode=disable", dbname)
	if connection, err = sqlx.Connect("postgres", params); err != nil {
		log.Fatalln(err)
	}

	return DB{connection}
}

func (d *DB) CreateImage(sourceURL string) (Image, error) {
	var (
		image Image
		query string
		err   error
		uid   uuid.UUID
	)

	uid, _ = uuid.NewV4()
	query = "INSERT INTO images (uuid, source_url) VALUES (:uuid, :source_url)"

	if _, err = d.connection.NamedExec(query, map[string]interface{}{
		"uuid":       uid.String(),
		"source_url": sourceURL,
	}); err != nil {
		return Image{}, err
	}

	query = "SELECT * FROM images WHERE uuid=$1"
	if err = d.connection.Get(&image, query, uid.String()); err != nil {
		return Image{}, err
	}

	return image, nil
}

func (d *DB) CreateContainer(image *Image) (Container, error) {
	var (
		container Container
		query     string
		err       error
		uid       uuid.UUID
	)

	uid, _ = uuid.NewV4()
	query = "INSERT INTO containers (uuid, image_id) VALUES (:uuid, :image_id)"

	if _, err = d.connection.NamedExec(query, map[string]interface{}{
		"status":   "active",
		"uuid":     uid.String(),
		"image_id": image.ID,
	}); err != nil {
		return container, err
	}

	query = "SELECT * FROM containers WHERE uuid=$1"
	if err = d.connection.Get(&container, query, uid.String()); err != nil {
		return container, err
	}

	return container, nil
}

func (d *DB) FindContainer(id string) (Container, error) {
	var (
		container Container
		err       error
	)

	if err = d.connection.Get(&container, "SELECT * FROM containers WHERE uuid=$1", id); err != nil {
		return container, err
	}

	return container, nil
}

func (d *DB) FindImage(id int) (Image, error) {
	var (
		image Image
		err   error
	)

	if err = d.connection.Get(&image, "SELECT * FROM images WHERE id=$1", id); err != nil {
		return image, err
	}

	return image, nil
}
