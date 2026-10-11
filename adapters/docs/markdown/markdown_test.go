package markdown

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPages(t *testing.T) {
	root := t.TempDir()
	for p, body := range map[string]string{
		"docs/guide.md":          "# The Guide\nhow to deploy the parser\n",
		"docs/adr/0001-kafka.md": "# Use Kafka\nwe chose kafka over rabbitmq\n",
		"README.md":              "# App\n",
		"docs/node_modules/x.md": "# no\n",
	} {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755)
		_ = os.WriteFile(filepath.Join(root, p), []byte(body), 0o644)
	}
	ps := Pages(root, DefaultDirs)
	if len(ps) != 3 || ps[1].ID != "docs/adr/0001-kafka.md" || ps[1].Title != "Use Kafka" || ps[1].Depth != 2 {
		t.Fatalf("pages %+v", ps)
	}
	hits := Search(ps, "kafka")
	if len(hits) != 1 || hits[0].Title != "Use Kafka" {
		t.Fatalf("search %+v", hits)
	}
	if hits := Search(ps, "deploy parser"); len(hits) != 1 || hits[0].ID != "docs/guide.md" {
		t.Fatalf("body search %+v", hits)
	}
	if _, err := inside(root, "../etc/passwd.md"); err == nil {
		t.Fatal("a path outside the project must be refused")
	}
}
