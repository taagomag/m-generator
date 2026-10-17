# Metrics Generator CLI

[![CI](https://github.com/taagomag/m-generator/actions/workflows/ci.yml/badge.svg)](https://github.com/taagomag/m-generator/actions/workflows/ci.yml)

A Go CLI for generating synthetic metrics and writing them to stdout or
publishing them to NATS JetStream, for example to load test a consumer.

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
one batch per second; set it with `generator.rate` or the `--rate` flag, which
takes precedence, using a count per second or minute such as `10/s`. Each
batch contains one sample for every configured metric and is sent to the
output as soon as it is generated.

### Outputs

Set `output.type` to one of:

- `stdout` (default): writes each batch as one JSON line, so it can be piped
  to `jq` or redirected to a file.
- `nats`: publishes each batch as JSON to `<subject_prefix>.batch` and waits
  for the JetStream acknowledgment. The stream is created if it does not
  exist; existing streams must capture `<subject_prefix>.>`. See
  [examples/nats-config.yaml](examples/nats-config.yaml).

```sh
go run ./cmd/metrics-generator generate -c examples/config.yaml --timeout 5s | jq .
```

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


## Development

CI runs both checks on every push to `main` and on pull requests. Run them
locally with the same golangci-lint version:

```sh
make lint
make test
```

## License

[MIT](LICENSE)
