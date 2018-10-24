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

type account struct {
	ID        int    `db:"id" json:"id"`
	UpdatedAt string `db:"updated_at" json:"updated_at"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

type User struct {
	ID        int    `db:"id" json:"id"`
	AccountID int    `db:"account_id" json:"account_id"`
	Username  string `db:"username" json:"username"`
	Password  string `db:"password" json:"-"`
	UpdatedAt string `db:"updated_at" json:"updated_at"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

type Image struct {
	ID        int    `db:"id" json:"id"`
	UUID      string `db:"uuid" json:"uuid"`
	AccountID int    `db:"account_id" json:"account_id"`
	SourceURL string `db:"source_url" json:"source_url"`
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
	query = "INSERT INTO images (uuid, source_url, account_id) VALUES (:uuid, :source_url, :account_id)"

	if _, err = d.connection.NamedExec(query, map[string]interface{}{
		"uuid":       uid.String(),
		"source_url": sourceURL,
		"account_id": user.AccountID,
	}); err != nil {
		return Image{}, err
	}

	query = "SELECT * FROM images WHERE uuid=$1"
	if err = d.connection.Get(&image, query, uid.String()); err != nil {
		return Image{}, err
	}

	return image, nil
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
		acct   account
		query  = "INSERT INTO users (username, password, account_id) VALUES ($1, $2, $3)"
		hashed []byte
	)

	if acct, err = d.CreateAccount(); err != nil {
		return User{}, err
	}

	if hashed, err = bcrypt.GenerateFromPassword([]byte(password), 8); err != nil {
		return User{}, err
	}

	if _, err = d.connection.Query(query, username, string(hashed), acct.ID); err != nil {
		return User{}, err
	}

	if err = d.connection.Get(&user, "SELECT * FROM users WHERE username=$1", username); err != nil {
		return User{}, err
	}

	return user, nil
}

func (d *DB) CreateAccount() (account, error) {
	var (
		err  error
		acct account
		id   int
	)

	r := d.connection.QueryRow("INSERT INTO accounts DEFAULT VALUES RETURNING id")
	if err = r.Scan(&id); err != nil {
		return account{}, err
	}

	if err = d.connection.Get(&acct, "SELECT * FROM accounts WHERE id=$1", id); err != nil {
		return account{}, err
	}

	return acct, nil
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
