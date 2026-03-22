# Observability stack (Grafana-first)

Everything is meant to be viewed in **Grafana** (http://localhost:3000): **Prometheus** metrics and **Tempo** traces. Prometheus has **no published host port**—only Grafana talks to it on the Docker network. OTLP from your app still hits **Tempo on `localhost:4317`**.

The helloworld binary usually runs **on the host** so you can develop without rebuilding images.

## Prerequisites

- Docker with Compose v2
- Helloworld reachable from the host on the **internal** metrics port (default `16667`) for Prometheus scraping
- Linux: `host.docker.internal` is set on the Prometheus service for scraping the host

## 1. Start the stack

From the **repository root**:

```bash
docker compose -f examples/observability/docker-compose.yml up -d
```

- **Grafana:** http://localhost:3000 — login `admin` / `admin`
- **Tempo OTLP (from host):** `127.0.0.1:4317` (gRPC), `4318` (HTTP)

## 2. Run helloworld with OTLP → Tempo

```bash
export HELLOWORLD_OTEL_EXPORTER_OTLP_ENDPOINT=127.0.0.1:4317
export HELLOWORLD_OTEL_EXPORTER_OTLP_INSECURE=true
# optional: HELLOWORLD_OTEL_SERVICE_NAME=helloworld
go run ./cmd/helloworld
```

Generate traffic:

```bash
./examples/observability/loadgen.sh
```

## 3. Use Grafana (metrics + traces)

1. **Dashboards** → **Helloworld HTTP** — Prometheus panels (request rate, in-flight).
2. **Explore** → choose datasource **Tempo** → **TraceQL** tab, for example:
   - `{ resource.service.name = "helloworld" }`
3. Optional: **Explore** → **Prometheus** to run PromQL against the same data as the dashboards.

You do not need a separate trace UI; Tempo’s query API is only used by Grafana on the internal network.

## 4. Environment reference

| Variable | Purpose |
|----------|---------|
| `HELLOWORLD_OTEL_EXPORTER_OTLP_ENDPOINT` | gRPC `host:port` (no `http://`). When set, tracing and `otelsql` instrumentation are enabled. |
| `HELLOWORLD_OTEL_EXPORTER_OTLP_INSECURE` | `true` for plaintext to local Tempo (default `true`). |
| `HELLOWORLD_OTEL_SERVICE_NAME` | Resource `service.name` (default `helloworld`). |
| `HELLOWORLD_OTEL_SAMPLE_RATIO` | `0.0`–`1.0` (default `1`). |
| `HELLOWORLD_INTERNAL_PORT` | Must match Prometheus scrape target if not `16667`. |

## 5. Notes

- **Internal HTTP** (`/metrics`, `/healthcheck`, `/status`) is a **separate listener** and is **not** wrapped with Chi/otel tracing middleware, so scrapes do not create server spans.
- HTTP instrumentation uses **[otelchi](https://github.com/riandyrn/otelchi)** with `WithChiRoutes` so span names use **route patterns**.
- DB calls use **[otelsql](https://github.com/XSAM/otelsql)** when OTLP is configured.
