package main

import (
	buildcommand "github.com/goxang/jig-plugins/adapters/builder/command"
	builddocker "github.com/goxang/jig-plugins/adapters/builder/docker"
	"github.com/goxang/jig-plugins/adapters/builder/golang"
	"github.com/goxang/jig-plugins/adapters/cache/redis"
	"github.com/goxang/jig-plugins/adapters/database/sql"
	"github.com/goxang/jig-plugins/adapters/debugger/delve"
	"github.com/goxang/jig-plugins/adapters/hosts/ssh"
	"github.com/goxang/jig-plugins/adapters/kv/consul"
	loadcommand "github.com/goxang/jig-plugins/adapters/loadgen/command"
	loadhttp "github.com/goxang/jig-plugins/adapters/loadgen/http"
	loadkv "github.com/goxang/jig-plugins/adapters/loadgen/kv"
	loadtool "github.com/goxang/jig-plugins/adapters/loadgen/tool"
	"github.com/goxang/jig-plugins/adapters/logs/loki"
	logsruntime "github.com/goxang/jig-plugins/adapters/logs/runtime"
	"github.com/goxang/jig-plugins/adapters/messaging/kafka"
	"github.com/goxang/jig-plugins/adapters/messaging/rabbitmq"
	"github.com/goxang/jig-plugins/adapters/metrics/prometheus"
	"github.com/goxang/jig-plugins/adapters/metrics/scrape"
	profcommand "github.com/goxang/jig-plugins/adapters/profiler/command"
	"github.com/goxang/jig-plugins/adapters/profiler/pprof"
	queryhttp "github.com/goxang/jig-plugins/adapters/query/http"
	rtdocker "github.com/goxang/jig-plugins/adapters/runtime/docker"
	"github.com/goxang/jig-plugins/adapters/runtime/kind"
	"github.com/goxang/jig-plugins/adapters/runtime/kubernetes"
	"github.com/goxang/jig-plugins/adapters/runtime/local"
	"github.com/goxang/jig-plugins/adapters/tracing/jaeger"
	"github.com/goxang/jig-plugins/adapters/tracing/tempo"
	"github.com/goxang/jig-plugins/adapters/tracing/zipkin"
	"github.com/goxang/jig/core"
)

// plugins are named by the component type jig.yaml uses; a name several kinds share is one plugin
// implementing each kind's contract. A probe is a nil value of the adapter's type: what it implements
// decides which optional capabilities the plugin provides.
var plugins = map[string][]served{
	"local":      {{core.KindRuntime, (*local.Runtime)(nil)}},
	"docker":     {{core.KindRuntime, (*rtdocker.Runtime)(nil)}, {core.KindBuilder, (*builddocker.Builder)(nil)}},
	"kubernetes": {{core.KindRuntime, (*kubernetes.Runtime)(nil)}},
	"kind":       {{core.KindRuntime, (*kind.Runtime)(nil)}},
	"go":         {{core.KindBuilder, (*golang.Builder)(nil)}},
	"command": {{core.KindBuilder, (*buildcommand.Builder)(nil)}, {core.KindProfiler, (*profcommand.Profiler)(nil)},
		{core.KindLoad, (*loadcommand.Gen)(nil)}},
	"prometheus": {{core.KindMetrics, (*prometheus.Metrics)(nil)}},
	"scrape":     {{core.KindMetrics, (*scrape.Metrics)(nil)}},
	"loki":       {{core.KindLogs, (*loki.Loki)(nil)}},
	"runtime":    {{core.KindLogs, (*logsruntime.Logs)(nil)}},
	"jaeger":     {{core.KindTracing, (*jaeger.Tracing)(nil)}},
	"tempo":      {{core.KindTracing, (*tempo.Tracing)(nil)}},
	"zipkin":     {{core.KindTracing, (*zipkin.Tracing)(nil)}},
	"pprof":      {{core.KindProfiler, (*pprof.Profiler)(nil)}},
	"delve":      {{core.KindDebugger, (*delve.Debugger)(nil)}},
	"sql":        {{core.KindDatabase, (*sql.DB)(nil)}},
	"redis":      {{core.KindCache, (*redis.Cache)(nil)}},
	"rabbitmq":   {{core.KindMessaging, (*rabbitmq.Rabbit)(nil)}},
	"kafka":      {{core.KindMessaging, (*kafka.Kafka)(nil)}},
	"consul":     {{core.KindKV, (*consul.KV)(nil)}},
	"http":       {{core.KindQuery, (*queryhttp.Query)(nil)}, {core.KindLoad, (*loadhttp.Gen)(nil)}},
	"kv":         {{core.KindLoad, (*loadkv.Gen)(nil)}},
	"k6":         {{core.KindLoad, (*loadtool.Gen)(nil)}},
	"vegeta":     {{core.KindLoad, (*loadtool.Gen)(nil)}},
	"ghz":        {{core.KindLoad, (*loadtool.Gen)(nil)}},
	"ssh":        {{core.KindHosts, (*ssh.SSH)(nil)}},
}
