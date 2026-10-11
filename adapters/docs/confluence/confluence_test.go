package confluence

import (
	"strings"
	"testing"
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
