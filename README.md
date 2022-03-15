## TODO

- [x] add accounts
- [x] track usage
- [] handle "exit"
- [] implement frontend

## Dev


```
```

#### Server

```
$ docker run --rm --name pg -e POSTGRES_USER=choxi -e POSTGRES_PASSWORD=password -p 5432:5432 postgres
$ docker exec -it pg psql -U choxi
> CREATE DATABASE dre_development;
CREATE DATABASE
> exit
$ exit
$ sql-migrate up
4 migrations applied
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
