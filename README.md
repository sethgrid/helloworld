# helloworld
## An example HTTP Server

What comes out of the box with this example implementation:

  - fully unit and unit-integration testable json http web server
  - able to spin up multiple running http servers in parallel during tests
  - able to assert against the server's logged contents
  - structured logging with `slog`
  - bubbling key:value error data upto `slog` for better structured error logging
  - fakes as test doubles, as a practical example against the need for mock and dependency injection frameworks
  - uses prometheus, and you can see logs and metrics via grafana and loki in docker
  - demonstrates building and testing via docker compose, see `make targets` for a list options
  
Some interesting choices:

  - The test server takes a variadic list of log writers, but only takes the first one. This is the closest thing to an optional parameter. It makes writing tests nice because you use the same new test server constructor, and you can optionally send in a logger. This would probably be better implemented as Options. 
  - each request places a logger into the context and there is a package with functions for making life easier for pulling the logger out of existing contexts / requests. This includes a strange thing I did where I put in a backup logger that is kinda gross, but because it is a variadic argument, you never see it nor have to use it.
  - There is a util directory. I know, I know. I still find value in a junk drawer. When it makes sense, things get pulled into their own package. And it is behind /internal/ anyway.
  - For migrations, I use `goose`, but I pulled those examples out for now.

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



