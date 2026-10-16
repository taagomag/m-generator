# Metrics Generator CLI

A Go CLI for generating synthetic metrics and writing them to a JSON file.

## CLI

Run the CLI from the repository root:

```sh
go run ./cmd/metrics-generator --help
go run ./cmd/metrics-generator --version
go run ./cmd/metrics-generator validate --config examples/config.yaml
```

The CLI provides Cobra's built-in `--version` flag and validates YAML
configuration files. `generate` collects batches for one minute by default,
then writes them as one JSON document to `output.json.file`, replacing any
existing file. Override the run time with `--timeout` (for example,
`--timeout 30s`). The default generation rate is one batch per second; configure
`generator.rate` using a count per second or minute, such as `10/s`. Each batch
contains one sample for every configured metric.
