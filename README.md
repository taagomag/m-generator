# Metrics Generator CLI

A Go CLI for generating synthetic metrics and writing them to a JSON file or
publishing them to NATS JetStream.

## CLI

Run the CLI from the repository root:

```sh
go run ./cmd/metrics-generator --help
go run ./cmd/metrics-generator --version
go run ./cmd/metrics-generator validate --config examples/config.yaml
```

The CLI provides Cobra's built-in `--version` flag and validates YAML
configuration files. `generate` runs for one minute by default; override the
run time with `--timeout` (for example, `--timeout 30s`). The default rate is
one batch per second; configure `generator.rate` using a count per second or
minute, such as `10/s`. Each batch contains one sample for every configured
metric. JSON output collects batches and writes one document at the end, while
NATS output publishes each batch as it is generated.

### Metric types

Each batch emits one sample per metric for a randomly chosen label
combination. State is tracked per series (metric + label values), following
Prometheus semantics:

| Type        | Value type  | Behavior                                                         |
|-------------|-------------|------------------------------------------------------------------|
| `counter`   | `increment` | Cumulative; grows by a random amount in `[min, max]` per sample  |
| `gauge`     | `random`    | Random value in `[min, max]`                                     |
| `histogram` | `random`    | Observes a random value in `[min, max]`; emits cumulative `_bucket{le}`, `_sum`, and `_count` samples |

Histograms use `buckets` when set, otherwise the Prometheus default buckets.
See [examples/config.yaml](examples/config.yaml) for every type.

