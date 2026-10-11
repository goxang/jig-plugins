package importers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// got runs an importer and returns its fragment as JSON, for substring checks.
func got(t *testing.T, f func(string, string) (map[string]any, error), path, dir string) string {
	t.Helper()
	frag, err := f(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(frag)
	return string(b)
}

func has(t *testing.T, s string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(s, w) {
			t.Errorf("missing %s in\n%s", w, s)
		}
	}
}

func hasNot(t *testing.T, s string, bad ...string) {
	t.Helper()
	for _, b := range bad {
		if strings.Contains(s, b) {
			t.Errorf("unexpected %s in\n%s", b, s)
		}
	}
}

func TestTaskRunners(t *testing.T) {
	d := t.TempDir()
	just := got(t, Just, write(t, d, "justfile", "set shell := [\"bash\", \"-c\"]\nversion := \"1\"\n\n# build the app\nbuild target='x':\n    go build\n_private:\n    true\n@test: build\n    go test\n"), d)
	has(t, just, `"build":{"help":"build the app","group":"just","steps":["just build \"$$@\""]}`, `"test":`)
	hasNot(t, just, "_private", "version", "shell")

	mk := got(t, Make, write(t, d, "Makefile", ".PHONY: build\nGO := go\nbuild: ## compile it\n\t$(GO) build\n# run the tests\ntest lint: build\n\tgo test\n%.o: %.c\n\tcc\n"), d)
	has(t, mk, `"build":{"help":"compile it"`, `"test":{"help":"run the tests"`, `"lint":`)
	hasNot(t, mk, "PHONY", `"GO"`, "%")

	write(t, d, "web/pnpm-lock.yaml", "")
	npm := got(t, NPM, write(t, d, "web/package.json", `{"scripts":{"prebuild":"rm -rf dist","build":"vite build","dev":"vite"}}`), d)
	has(t, npm, `"steps":["cd web `, `pnpm run build -- \"$$@\""]`, `"dev":`)
	hasNot(t, npm, "prebuild")

	pf := got(t, Procfile, write(t, d, "Procfile", "web: bundle exec puma -p $PORT\n# comment\nworker: sidekiq\n"), d)
	has(t, pf, `"web":{"run":{"command":["sh","-c","bundle exec puma -p $$PORT"]}}`, `"worker":`)

	env := got(t, Dotenv, write(t, d, ".env", "# x\nexport A=1\nB=\"two words\"\nC=3 # note\nD='$x'\n"), d)
	has(t, env, `"A":"1"`, `"B":"two words"`, `"C":"3"`, `"D":"$$x"`)
}

func TestCI(t *testing.T) {
	d := t.TempDir()
	gl := got(t, GitLabCI, write(t, d, ".gitlab-ci.yml", `
variables: { GO: "1.26" }
default: { before_script: [echo setup] }
stages: [test, deploy]
.base: { stage: test, variables: { CGO_ENABLED: "0" } }
unit:
  extends: .base
  script: [go test ./..., [go vet ./...]]
deploy:
  stage: deploy
  trigger: other/project
`), d)
	has(t, gl, `"ci-unit"`, `stage test`, `export CGO_ENABLED='0'\nexport GO='1.26'\necho setup\ngo test ./...\ngo vet ./...`)
	hasNot(t, gl, "ci-deploy", "ci-.base")

	write(t, d, ".github/workflows/ci.yml", `
name: CI
env: { A: "1", T: "${{ secrets.T }}" }
jobs:
  test:
    steps:
      - uses: actions/checkout@v4
      - name: unit
        run: go test ./...
        working-directory: app
      - run: echo ${{ github.sha }}
  only-uses:
    steps: [{ uses: actions/setup-go@v5 }]
`)
	gh := got(t, GitHubActions, filepath.Join(d, ".github/workflows"), d)
	has(t, gh, `"gh-ci-test"`, `skips actions/checkout@v4`, `# unit\ncd app\nexport A='1'\ngo test ./...`)
	hasNot(t, gh, "only-uses", "secrets.T", "github.sha")
}

func TestObservability(t *testing.T) {
	d := t.TempDir()
	g := got(t, Grafana, write(t, d, "dash/api.json", `{"dashboard":{"title":"API Overview",
 "templating":{"list":[{"name":"service","type":"query","query":"label_values(up, service)","current":{"value":"api"}},
   {"name":"env","type":"custom","query":"dev, prod","includeAll":true}]},
 "panels":[{"type":"row","title":"Traffic","panels":[
   {"type":"timeseries","title":"rps","gridPos":{"w":8},"fieldConfig":{"defaults":{"unit":"reqps","thresholds":{"steps":[{"value":null},{"value":100},{"value":500}]}}},
    "targets":[{"expr":"sum(rate(x{service=~\"$service\"}[1m]))","legendFormat":"{{service}}"}]}]},
   {"type":"text","title":"notes"}]}}`), d)
	has(t, g, `"api-overview"`, `"row":"Traffic"`, `"unit":"/s"`, `"warn":100`, `"crit":500`, `"width":8`, `"legend":"{{service}}"`,
		`"service":{"default":"api","label":"service","query":"up"}`, `"values":["dev","prod"]`)
	hasNot(t, g, "notes")

	pr := got(t, PrometheusRules, write(t, d, "rules.yml", `
groups:
  - name: g
    rules:
      - record: x:sum
        expr: sum(x)
      - alert: HighLatency
        expr: histogram_quantile(0.99, sum by (le) (rate(h[5m]))) > 0.5
        labels: { severity: critical }
      - alert: Down
        expr: up == 0
      - alert: FewConsumers
        expr: sum(consumers) < 2
`), d)
	has(t, pr, `"crit":0.5`, `"query":"histogram_quantile(0.99, sum by (le) (rate(h[5m])))"`, `"query":"count(up == 0)","source":"${METRICS:-prometheus}","warn":1`,
		`"below":true`)
	hasNot(t, pr, "x:sum")

	oa := got(t, OpenAPI, write(t, d, "openapi.yaml", `
openapi: 3.0.0
info: { title: Pet Store }
servers: [{ url: "http://localhost:8080/v1" }]
paths:
  /pets/{id}:
    get: { operationId: getPet, summary: one pet, tags: [pets] }
    delete: { operationId: deletePet }
`), d)
	has(t, oa, `"pet-store":{"addr":"${PET_STORE_URL:-http://localhost:8080/v1}","kind":"query","type":"http"}`,
		`"getpet":{"group":"pet-store/pets","help":"one pet","query":"GET /pets/{{id}}","source":"pet-store"}`)
	hasNot(t, oa, "deletepet")
}
