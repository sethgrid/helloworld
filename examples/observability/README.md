# Observability stack (Grafana-first)

Everything is meant to be viewed in **Grafana** at http://localhost:3000 (login: `admin` / `admin`):

- **Prometheus** metrics — pre-built dashboard + Explore
- **Tempo** traces — Explore → TraceQL
- **Loki** logs — Explore → LogQL (helloworld JSON logs, including random errors the handler emits)

Prometheus has no published host port — only Grafana reaches it on the Docker network. OTLP from your app hits Tempo on `localhost:4317`. Logs are piped from the host process into a file that promtail ships to Loki.

The helloworld binary runs **on the host** so you can develop without rebuilding images.

## Prerequisites

- Docker with Compose v2
- `HELLOWORLD_OTEL_EXPORTER_OTLP_ENDPOINT` is ignored and no traces are emitted
F- Linux: `host.docker.internal` is set on the Prometheus service for scraping the host

## 1. Start MySQL

From the **repository root**:

```bash
docker compose up -d mysql
```

Wait for it to be healthy (the healthcheck polls `mysqladmin ping`):

```bash
docker compose ps mysql
```

## 2. Start the observability stack

```bash
docker compose -f examples/observability/docker-compose.yml up -d
```

- **Grafana:** http://localhost:3000 — login `admin` / `admin`
- **Tempo OTLP (from host):** `127.0.0.1:4317` (gRPC), `4318` (HTTP)
- **Prometheus** is internal-only (no published host port); use Grafana to query it

## 3. Run helloworld with OTLP → Tempo and logs → Loki

Logs are shipped to Loki by tailing a file. Create it first (gitignored):

```bash
touch logs/helloworld.logs
```

Then run helloworld, tee-ing stdout to that file:

```bash
export HELLOWORLD_OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317
export HELLOWORLD_OTEL_EXPORTER_OTLP_INSECURE=true
# optional: HELLOWORLD_OTEL_SERVICE_NAME=helloworld
go run ./cmd/helloworld 2>&1 | tee logs/helloworld.logs
```

Generate traffic in a second terminal (runs until Ctrl+C; hits the public port `16666` by default):

```bash
./examples/observability/loadgen.sh
```

To use a different base URL: `HELLOWORLD_URL=http://127.0.0.1:16666 ./examples/observability/loadgen.sh`

## 4. Open Grafana

Go to **http://localhost:3000** and log in with `admin` / `admin`.

- **Dashboards → Helloworld HTTP** — Prometheus panels (request rate, in-flight, error rate).
- **Explore → Tempo** → TraceQL tab:
  ```
  { resource.service.name = "helloworld" }
  ```
- **Explore → Loki** → LogQL tab. The handler deliberately emits random 500 errors — useful for seeing error logs alongside traces:
  ```
  {job="helloworld"}
  ```
  Filter to errors only:
  ```
  {job="helloworld", level="ERROR"}
  ```

You do not need a separate trace or log UI; all three signal types are queried by Grafana on the internal Docker network.

## 5. Environment reference

| Variable | Purpose |
|----------|---------|
| `HELLOWORLD_OTEL_EXPORTER_OTLP_ENDPOINT` | gRPC `host:port` (no `http://`). When set, tracing and `otelsql` instrumentation are enabled. |
| `HELLOWORLD_OTEL_EXPORTER_OTLP_INSECURE` | `true` for plaintext to local Tempo (default `true`). |
| `HELLOWORLD_OTEL_SERVICE_NAME` | Resource `service.name` (default `helloworld`). |
| `HELLOWORLD_OTEL_SAMPLE_RATIO` | `0.0`–`1.0` (default `1`). |
| `HELLOWORLD_INTERNAL_PORT` | Must match Prometheus scrape target if not `16667`. |

## 6. Notes

- **Internal HTTP** (`/metrics`, `/healthcheck`, `/status`) is a **separate listener** and is **not** wrapped with Chi/otel tracing middleware, so scrapes do not create server spans.
- HTTP instrumentation uses **[otelchi](https://github.com/riandyrn/otelchi)** with `WithChiRoutes` so span names use **route patterns**.
- DB calls use **[otelsql](https://github.com/XSAM/otelsql)** when OTLP is configured.
