# helloworld
## An example HTTP Server

### Initial Development

Install `make`, `docker`, `docker compose`, and `go`.
Recommended: `alias dc='docker compose'`

Run:
```
# terminal 1
dc up mysql
# or
docker compose up -d mysql

# terminal 2
source settings.env
go run cmd/helloworld/main.go

# terminal 3 (note, different port for internal endpoints)
curl localhost:16667/healthcheck
200 OK
```

# Local metrics

if you would like to see metrics and log grabbing in action:
```
dc up -d mysql grafana
go run cmd/helloworld/main.go >> ./logs/helloworld.logs
# open localhost:3000
```

Metrics are polled from the internal port /metrics to prometheus where grafana then displays data. Similarly, promtail follows either dev or production logs and pushes them into loki, where grafana can then display the results.

This requires prometheus and promtail to be able to reach the host system outsidet he docker container.


### Testing

Generally, you will just run `go test ./...` and verify that the unit tests are working. However, we can also run integration tests to make sure that the app still
can work with the db and sign in as expected.

There are two types of integration tests we are using. Integration and Unit-Integration. Integration runs against the service binary the real database. Unit-Integration runs a headless browser from unit tests.

```
# may need to prefix each of these with "sudo" if you get permission errors

make test                       # will run the unit tests; same as go test ./...
sudo make test-integration           # will tear down the db, reseed it, re-build helloworld, and then test it blackbox style
sudo make test-integration-docker    # runs everything in docker, like CI/CD does
```


### TODOs

 - [ ] timing metrics to db and to any API
 - [ ] Make a testcase showing grpc handling
 - [ ] Make a testcase showing graphql handling
 - [ ] status page and external monitoring
 - [ ] rate limiting
 - [ ] UI route handling? or just keep this as a JSON api server?



