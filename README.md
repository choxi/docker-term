## TODO

- [x] add accounts
- [x] track usage
- [] handle "exit"
- [] implement frontend

## Dev

#### Server

```
$ go run main.go
```

#### Migrations

```
$ sql-migrate up
```


## Useful Docker Commands

Kill all containers

```bash
$ docker kill $(docker ps -q)
```

Delete all stopped containers

```
$ docker rm $(docker ps -a -q)
```

Delete all images

```
$ docker rmi $(docker images -q)
