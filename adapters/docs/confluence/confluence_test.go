package confluence

import (
	"strings"
	"testing"

	"github.com/goxang/jig-plugins/internal/forge"
)

func TestConvert(t *testing.T) {
	md := "# Deploy\n\nRun **make** then `jig up`.\nSee [guide](http://x/g).\n\n- one\n- two\n\n```\necho <hi>\n```\n"
	st := Storage(md)
	for _, want := range []string{"<h1>Deploy</h1>", "<strong>make</strong>", "<code>jig up</code>", `<a href="http://x/g">guide</a>`,
		"<ul><li>one</li><li>two</li></ul>", "<![CDATA[echo <hi>\n]]>"} {
		if !strings.Contains(st, want) {
			t.Errorf("storage misses %s:\n%s", want, st)
		}
	}
	back := Markdown(st)
	for _, want := range []string{"# Deploy", "**make**", "`jig up`", "[guide](http://x/g)", "- one\n- two", "```\necho <hi>\n```"} {
		if !strings.Contains(back, want) {
			t.Errorf("markdown misses %q:\n%s", want, back)
		}
	}
}

func TestTree(t *testing.T) {
	page := func(id, title string, anc ...string) content {
		c := content{ID: id, Title: title}
		for _, a := range anc {
			c.Ancestors = append(c.Ancestors, struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			}{ID: a})
		}
		return c
	}
	cf := &Confluence{Client: &forge.Client{}}
	rows := cf.tree([]content{page("3", "b child", "1"), page("1", "Home"), page("2", "a child", "1"), page("4", "deep", "1", "2")})
	var got []string
	for _, r := range rows {
		got = append(got, r.Title)
	}
	want := []string{"└─ Home", "   ├─ a child", "   │  └─ deep", "   └─ b child"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q", got)
	}
	if k := spaceKeys("[PROD SWIT]"); len(k) != 2 || k[1] != "SWIT" {
		t.Fatalf("spaceKeys = %q", k)
	}
}
