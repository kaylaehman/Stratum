# Stratum Observability

Stratum exposes a Prometheus-compatible scrape endpoint at `GET /metrics` on the
same port as the API (default `:8080`).  The endpoint is public (no auth required)
so Prometheus can scrape it without needing a service account.

If you want to restrict access, put a reverse proxy in front and whitelist only
your Prometheus instance's IP.

---

## Metrics exposed

| Metric | Type | Description |
|---|---|---|
| `stratum_remediation_proposals_total{status}` | Gauge | Count of agentic-remediation proposals by status (`proposed`, `approved`, `rejected`, `executed`, `failed`). All label values are always present (zero-valued) so Grafana graphs are never empty on a fresh install. |
| `stratum_ws_clients` | Gauge | Number of currently connected WebSocket clients. |
| `stratum_ws_messages_dropped_total` | Counter | Cumulative WebSocket messages dropped because a slow consumer's send buffer was full. A sustained non-zero rate indicates clients that cannot keep up with the event stream. |
| `go_*` | various | Standard Go runtime metrics (goroutines, GC, memory). |
| `process_*` | various | Process-level metrics (RSS, open FDs, CPU). |

Future waves will add:

- `stratum_node_poll_duration_seconds{node_id}` — node-poll latency histogram
- `stratum_ssh_pool_errors_total{node_id}` — SSH pool error counters
- `stratum_http_request_duration_seconds{route,method,status}` — API latency

---

## Prometheus scrape config

Add the following job to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: stratum
    static_configs:
      - targets: ["<your-stratum-host>:8080"]
    metrics_path: /metrics
    scrape_interval: 30s
```

Replace `<your-stratum-host>` with the hostname or IP of your Stratum instance.
If Stratum is running in Docker Compose alongside Prometheus, use the service
name (e.g. `stratum-backend:8080`).

---

## Grafana dashboard

A starter dashboard JSON is shipped at `assets/grafana/stratum-dashboard.json`.

To import it:

1. Open Grafana, navigate to **Dashboards > Import**.
2. Click **Upload JSON file** and select `assets/grafana/stratum-dashboard.json`.
3. Select your Prometheus datasource when prompted.
4. Click **Import**.

The dashboard includes panels for:

- Remediation proposal counts by status (time series + stat cards)
- Active WebSocket clients and message drop rate
- Go goroutine count, heap memory, and process RSS

---

## Single-binary distribution

See [single-binary.md](single-binary.md) for how to build a single binary that
serves both the API and the frontend SPA (using the `embed` build tag).
