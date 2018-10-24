package db

import (
	"database/sql"
	"dre/utils"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

type Container struct {
	ID        int    `db:"id" json:"id"`
	UUID      string `db:"uuid" json:"uuid"`
	ImageID   int    `db:"image_id" json:"image_id"`
	UpdatedAt string `db:"updated_at" json:"updated_at"`
	CreatedAt string `db:"created_at" json:"created_at"`
	database  *DB
	run       run
}

type run struct {
	ID          int            `db:"id" json:"id"`
	StartedAt   string         `db:"started_at" json:"started_at"`
	EndedAt     sql.NullString `db:"ended_at" json:"ended_at"`
	UpdatedAt   string         `db:"updated_at" json:"updated_at"`
	CreatedAt   string         `db:"created_at" json:"created_at"`
	ContainerID int            `db:"container_id" json:"container_id"`
}

func (d *DB) FindContainer(id string) (Container, error) {
	var (
		container = Container{database: d}
		err       error
	)

	if err = d.connection.Get(&container, "SELECT * FROM containers WHERE uuid=$1", id); err != nil {
		return Container{}, err
	}

	return container, nil
}

func (d *DB) CreateContainer(image *Image) (Container, error) {
	var (
		container = Container{database: d}
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
		return Container{}, errors.Wrap(err, "")
	}

	query = "SELECT * FROM containers WHERE uuid=$1"
	if err = d.connection.Get(&container, query, uid.String()); err != nil {
		return Container{}, errors.Wrap(err, "")
	}

	return container, nil
}

func (c *Container) Start() error {
	var (
		err   error
		id    int
		query = "INSERT INTO runs (container_id, started_at) VALUES ($1, now()) RETURNING id"
		r     run
		row   *sql.Row
	)

	row = c.database.connection.QueryRow(query, c.ID)
	if err = row.Scan(&id); err != nil {
		return utils.Error(err, "db: run not created")
	}

	if err = c.database.connection.Get(&r, "SELECT * FROM runs WHERE id=$1", id); err != nil {
		return utils.Error(err, "db: run not found")
	}

	c.run = r

	return nil
}

func (c *Container) End() error {
	var (
		err   error
		query = "UPDATE runs SET ended_at=now() WHERE ID=$1"
	)

	if _, err = c.database.connection.Query(query, c.run.ID); err != nil {
		return utils.Error(err, "db: run not updated")
	}

	return nil
}
