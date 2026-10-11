package helm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHelm puts a helm on PATH that answers list, history and get values.
func fakeHelm(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$1 $2" in
"list -o") echo '[{"name":"api","namespace":"prod","revision":"3","updated":"2026-01-02 15:04:05.123 +0000 UTC","status":"deployed","chart":"api-1.2.0","app_version":"1.2.0"}]' ;;
"history api") echo '[{"revision":2,"status":"superseded","chart":"api-1.1.0"},{"revision":3,"status":"deployed","chart":"api-1.2.0"}]' ;;
"get values") case "$*" in *"--revision 2"*) echo "replicas: 2" ;; *) echo "replicas: 3" ;; esac ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "helm"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestHelm(t *testing.T) {
	fakeHelm(t)
	ctx := context.Background()
	h := helm{namespace: "prod"}
	rs, err := h.Releases(ctx)
	if err != nil || len(rs) != 1 || rs[0].Revision != 3 || rs[0].Updated.Year() != 2026 {
		t.Fatalf("releases %+v %v", rs, err)
	}
	hist, err := h.History(ctx, "api")
	if err != nil || hist[0].Revision != 3 {
		t.Fatalf("history %+v %v", hist, err)
	}
	d, err := h.Diff(ctx, "api", 0, 0)
	if err != nil || !strings.Contains(d, "-replicas: 2") || !strings.Contains(d, "+replicas: 3") {
		t.Fatalf("diff %q %v", d, err)
	}
}
