## TODO

- [x] add accounts
- [] track usage
- [] implement frontend

## Useful Docker Commands

Kill all containers

```bash
$ docker rm $(docker ps -q) 
```

Delete all stopped containers

```
$ docker rm $(docker ps -a -q)
```

Delete all images

```
$ docker rmi $(docker images -q)
