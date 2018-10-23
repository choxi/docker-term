package db

import (
	"fmt"
	"log"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

type Credentials struct {
	Password string `json:"password", db:"password"`
	Username string `json:"username", db:"username"`
}

type User struct {
	ID        int    `db:"id" json:"id"`
	Username  string `db:"username" json:"username"`
	Password  string `db:"password" json:"-"`
	UpdatedAt string `db:"updated_at" json:"updated_at"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

type Image struct {
	ID        int    `db:"id" json:"id"`
	UUID      string `db:"uuid" json:"uuid"`
	UserID    int    `db:"user_id" json:"user_id"`
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

func (d *DB) CreateImage(user User, sourceURL string) (Image, error) {
	var (
		image Image
		query string
		err   error
		uid   uuid.UUID
	)

	uid, _ = uuid.NewV4()
	query = "INSERT INTO images (uuid, source_url, user_id) VALUES (:uuid, :source_url, :user_id)"

	if _, err = d.connection.NamedExec(query, map[string]interface{}{
		"uuid":       uid.String(),
		"source_url": sourceURL,
		"user_id":    user.ID,
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

func (d *DB) FindUser(username string) (User, error) {
	var (
		user User
		err  error
	)

	if err = d.connection.Get(&user, "SELECT * FROM users WHERE username=$1", username); err != nil {
		return User{}, err
	}

	return user, nil
}

func (d *DB) CreateUser(username string, password string) (User, error) {
	var (
		err    error
		user   User
		query  = "INSERT INTO users (username, password) VALUES ($1, $2)"
		hashed []byte
	)

	if hashed, err = bcrypt.GenerateFromPassword([]byte(password), 8); err != nil {
		return user, err
	}

	if _, err = d.connection.Query(query, username, string(hashed)); err != nil {
		return user, err
	}

	return user, nil
}

func (d *DB) SignInUser(username string, password string) (User, error) {
	var (
		err  error
		user User
	)

	if err = d.connection.Get(&user, "SELECT * FROM users WHERE username=$1", username); err != nil {
		return User{}, err
	}

	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return User{}, err
	}

	return user, nil
}

var secret = []byte("PANCAKES")

func CreateToken(user *User) (string, error) {
	var (
		token       *jwt.Token
		tokenString string
		err         error
	)

	token = jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": user.Username,
	})

	if tokenString, err = token.SignedString(secret); err != nil {
		return "", err
	}

	return tokenString, nil
}

func (d *DB) AuthenticateToken(tokenString string) (User, error) {
	var (
		token    *jwt.Token
		err      error
		ok       bool
		claims   jwt.MapClaims
		username string
		user     User
	)

	token, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok = token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return secret, nil
	})

	if err != nil {
		return User{}, err
	}

	if claims, ok = token.Claims.(jwt.MapClaims); !ok || !token.Valid {
		return User{}, fmt.Errorf("Invalid authentication token")
	}

	if username, ok = claims["username"].(string); !ok {
		return User{}, err
	}

	if user, err = d.FindUser(username); err != nil {
		return User{}, err
	}

	return user, nil
}
