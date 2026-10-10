# jig-plugins

The official plugins for [jig](https://github.com/goxang/jig): runtimes, builders, databases,
brokers, metrics, logs, traces, profilers, debuggers, load generators and hosts.

```bash
go install github.com/goxang/jig-plugins/cmd/jig-std@latest
jig-std install     # writes a manifest per plugin to ~/.config/jig/plugins
jig-std list        # what each plugin serves
```

`jig-std` is one binary holding every Go plugin; jig starts `jig-std <name>` when a component's
type names that plugin. A name shared by several kinds (`docker`, `command`, `http`) is one plugin
implementing each kind's contract.

## Layout

- `cmd/jig-std/` the binary and its plugin table
- `adapters/<kind>/<type>/` the adapters, served through `contracts.Serve`
- `internal/` helpers the adapters share (shell, HTTP, kube contexts)

## Adding a plugin

Write the adapter against `github.com/goxang/jig/core`, register it with `plugin.Register`, and add
it to the table in `cmd/jig-std/plugins.go`. `go test ./cmd/jig-std` checks it serves its contract.
