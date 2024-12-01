## Integration Tests

Integration tests can be ran locally for speed or against docker to ensure a consistent state.

When developing locally, run the integration tests locally. Pull requests should always run the integration tests inside docker.


### Local
```
make test-integration
```
or
```
docker compose build integration
docker compose run --rm integration
```

If your changes to the integration tests are not being picked up, you can optionally bust the build cache at the go build stage with:

```
docker compose build --build-arg CACHEBUST=$(date +%s) integration
```

### Docker

```
make test-integration-docker
```