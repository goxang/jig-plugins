// Package confluence is Confluence as docs/1: CQL search, pages read as Markdown, written from
// Markdown, and the page tree, over the REST API (Cloud's /wiki or a Server base URL).
package confluence

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/goxang/jig-plugins/internal/forge"
	"github.com/goxang/jig-plugins/internal/frames"
	"github.com/goxang/jig-plugins/internal/httpx"
	"github.com/goxang/jig/sdk"
)

type Confluence struct {
	*forge.Client
	space string
}

// Open reads plugins.confluence: url (https://org.atlassian.net/wiki), user (Cloud: the account
// email), token (CONFLUENCE_TOKEN or ATLASSIAN_TOKEN) and space (the default space key).
func Open(c *sdk.Ctx) (*Confluence, error) {
	base := strings.TrimSuffix(forge.Config(c, "url"), "/")
	if base == "" {
		return nil, sdk.Unavailable("confluence has no address", "set plugins.confluence.url (https://your-org.atlassian.net/wiki)")
	}
	ep := httpx.Endpoint{Headers: map[string]string{"Accept": "application/json"}}
	tok := forge.Token(c, "CONFLUENCE_TOKEN", "ATLASSIAN_TOKEN")
	if u := forge.Config(c, "user"); u != "" {
		ep.User, ep.Password = u, tok
	} else {
		ep.Token = tok
	}
	return &Confluence{Client: &forge.Client{Base: base, Endpoint: ep}, space: forge.Config(c, "space")}, nil
}

type content struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Space   struct{ Key string }
	Version struct {
		Number int       `json:"number"`
		When   time.Time `json:"when"`
	} `json:"version"`
	Ancestors []struct{ Title string } `json:"ancestors"`
	Body      struct {
		Storage struct{ Value string } `json:"storage"`
	} `json:"body"`
	Links struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

type page struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Path    string    `json:"path"`
	Updated time.Time `json:"updated"`
	URL     string    `json:"url"`
}

func (cf *Confluence) page(c content) page {
	path := []string{c.Space.Key}
	for _, a := range c.Ancestors {
		path = append(path, a.Title)
	}
	return page{ID: c.ID, Title: c.Title, Path: strings.Join(path, " / "), Updated: c.Version.When, URL: cf.Base + c.Links.WebUI}
}

func (cf *Confluence) list(ctx context.Context, path string) ([]page, error) {
	var res struct{ Results []content }
	if err := cf.Get(ctx, path, &res); err != nil {
		return nil, err
	}
	out := make([]page, 0, len(res.Results))
	for _, r := range res.Results {
		out = append(out, cf.page(r))
	}
	return out, nil
}

var (
	tagRE   = regexp.MustCompile(`(?s)<[^>]+>`)
	codeRE  = regexp.MustCompile(`(?s)<ac:structured-macro ac:name="code".*?<!\[CDATA\[(.*?)\]\]>.*?</ac:structured-macro>`)
	blankRE = regexp.MustCompile(`\n{3,}`)
	toMD    = []struct {
		re  *regexp.Regexp
		out string
	}{
		{regexp.MustCompile(`(?s)<h([1-6])[^>]*>(.*?)</h[1-6]>`), "\n\n${H}$2\n\n"},
		{regexp.MustCompile(`(?s)<li[^>]*>(.*?)</li>`), "\n- $1"},
		{regexp.MustCompile(`(?s)<(strong|b)>(.*?)</(strong|b)>`), "**$2**"},
		{regexp.MustCompile(`(?s)<(em|i)>(.*?)</(em|i)>`), "_$2_"},
		{regexp.MustCompile(`(?s)<code>(.*?)</code>`), "`$1`"},
		{regexp.MustCompile(`(?s)<a [^>]*href="([^"]*)"[^>]*>(.*?)</a>`), "[$2]($1)"},
		{regexp.MustCompile(`<br\s*/?>`), "\n"},
		{regexp.MustCompile(`(?s)</?(p|ul|ol|div|table|tr)[^>]*>`), "\n\n"},
		{regexp.MustCompile(`(?s)</t[dh]>`), " | "},
	}
)

// Markdown is a storage-format page as Markdown: headings, lists, emphasis, code, links and
// paragraphs; other macros keep only their text.
func Markdown(storage string) string {
	// code bodies are set aside so tag stripping leaves them as they are
	var code []string
	s := codeRE.ReplaceAllStringFunc(storage, func(m string) string {
		code = append(code, codeRE.FindStringSubmatch(m)[1])
		return fmt.Sprintf("\n\n\x00%d\x00\n\n", len(code)-1)
	})
	for _, r := range toMD {
		if strings.Contains(r.out, "${H}") {
			s = r.re.ReplaceAllStringFunc(s, func(m string) string {
				g := r.re.FindStringSubmatch(m)
				return "\n\n" + strings.Repeat("#", int(g[1][0]-'0')) + " " + g[2] + "\n\n"
			})
			continue
		}
		s = r.re.ReplaceAllString(s, r.out)
	}
	s = html.UnescapeString(tagRE.ReplaceAllString(s, ""))
	for i, c := range code {
		s = strings.Replace(s, fmt.Sprintf("\x00%d\x00", i), "```\n"+strings.TrimRight(c, "\n")+"\n```", 1)
	}
	return strings.TrimSpace(blankRE.ReplaceAllString(s, "\n\n"))
}

var (
	mdHeading = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	mdBold    = regexp.MustCompile(`\*\*(.+?)\*\*`)
	mdCode    = regexp.MustCompile("`([^`]+)`")
	mdLink    = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

// Storage is Markdown as Confluence storage format: headings, lists, fenced code, bold, code and links.
func Storage(md string) string {
	var b strings.Builder
	inList, inCode := false, false
	var para []string
	flush := func() {
		if len(para) > 0 {
			b.WriteString("<p>" + inline(strings.Join(para, " ")) + "</p>")
			para = nil
		}
	}
	for _, l := range strings.Split(md, "\n") {
		if strings.HasPrefix(l, "```") {
			flush()
			if inCode {
				b.WriteString("]]></ac:plain-text-body></ac:structured-macro>")
			} else {
				b.WriteString(`<ac:structured-macro ac:name="code"><ac:plain-text-body><![CDATA[`)
			}
			inCode = !inCode
			continue
		}
		if inCode {
			b.WriteString(l + "\n")
			continue
		}
		item, isItem := strings.CutPrefix(strings.TrimLeft(l, " "), "- ")
		if !isItem {
			item, isItem = strings.CutPrefix(strings.TrimLeft(l, " "), "* ")
		}
		if inList && !isItem {
			b.WriteString("</ul>")
			inList = false
		}
		switch m := mdHeading.FindStringSubmatch(l); {
		case m != nil:
			flush()
			n := len(m[1])
			fmt.Fprintf(&b, "<h%d>%s</h%d>", n, inline(m[2]), n)
		case isItem:
			flush()
			if !inList {
				b.WriteString("<ul>")
				inList = true
			}
			b.WriteString("<li>" + inline(item) + "</li>")
		case strings.TrimSpace(l) == "":
			flush()
		default:
			para = append(para, strings.TrimSpace(l))
		}
	}
	flush()
	if inList {
		b.WriteString("</ul>")
	}
	return b.String()
}

func inline(s string) string {
	s = html.EscapeString(s)
	s = mdBold.ReplaceAllString(s, "<strong>$1</strong>")
	s = mdCode.ReplaceAllString(s, "<code>$1</code>")
	return mdLink.ReplaceAllString(s, `<a href="$2">$1</a>`)
}

func Register(p *sdk.Plugin) {
	p.Source("docs.search", func(c *sdk.Ctx, in struct {
		Query string `json:"query" desc:"words, or CQL"`
	}) (*sdk.Frame, error) {
		cf, err := Open(c)
		if err != nil {
			return nil, err
		}
		cql := in.Query
		if !strings.ContainsAny(cql, "=~") {
			cql = "type = page"
			if in.Query != "" {
				cql += fmt.Sprintf(" AND text ~ %q", in.Query)
			}
			if cf.space != "" {
				cql += " AND space = " + cf.space
			}
			cql += " ORDER BY lastmodified DESC"
		}
		rows, err := cf.list(c, "/rest/api/content/search?limit=50&expand=space,version,ancestors&cql="+url.QueryEscape(cql))
		return frames.Of(rows), err
	}, sdk.Cache("30s"))

	p.Source("docs.read", func(c *sdk.Ctx, in struct {
		ID string `json:"id" jig:"required"`
	}) (*sdk.Frame, error) {
		cf, err := Open(c)
		if err != nil {
			return nil, err
		}
		var ct content
		if err := cf.Get(c, "/rest/api/content/"+url.PathEscape(in.ID)+"?expand=body.storage,version", &ct); err != nil {
			return nil, err
		}
		type doc struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Markdown string `json:"markdown"`
		}
		return frames.Of([]doc{{ct.ID, ct.Title, Markdown(ct.Body.Storage.Value)}}), nil
	})

	p.Action("docs.write", sdk.EffectWrite, func(c *sdk.Ctx, in struct {
		ID       string `json:"id" desc:"the page to update; empty makes a new one"`
		Title    string `json:"title" jig:"required"`
		Markdown string `json:"markdown" jig:"required"`
		Parent   string `json:"parent" desc:"parent page id for a new page"`
		Space    string `json:"space" desc:"space key (default plugins.confluence.space)"`
	}) (*sdk.Result, error) {
		cf, err := Open(c)
		if err != nil {
			return nil, err
		}
		body := map[string]any{"type": "page", "title": in.Title,
			"body": map[string]any{"storage": map[string]string{"value": Storage(in.Markdown), "representation": "storage"}}}
		var out content
		if in.ID != "" {
			var cur content
			if err := cf.Get(c, "/rest/api/content/"+url.PathEscape(in.ID)+"?expand=version", &cur); err != nil {
				return nil, err
			}
			body["version"] = map[string]int{"number": cur.Version.Number + 1}
			err = cf.Send(c, "PUT", "/rest/api/content/"+url.PathEscape(in.ID), body, &out)
		} else {
			space := in.Space
			if space == "" {
				space = cf.space
			}
			if space == "" {
				return nil, fmt.Errorf("which space? set space or plugins.confluence.space")
			}
			body["space"] = map[string]string{"key": space}
			if in.Parent != "" {
				body["ancestors"] = []map[string]string{{"id": in.Parent}}
			}
			err = cf.Send(c, "POST", "/rest/api/content", body, &out)
		}
		if err != nil {
			return nil, err
		}
		return &sdk.Result{Message: "saved " + out.Title, Outputs: sdk.Args{"id": out.ID, "url": cf.Base + out.Links.WebUI}}, nil
	})

	p.Source("docs.tree", func(c *sdk.Ctx, in struct {
		Parent string `json:"parent" desc:"a page id (default the space's top pages)"`
	}) (*sdk.Frame, error) {
		cf, err := Open(c)
		if err != nil {
			return nil, err
		}
		path := "/rest/api/content/" + url.PathEscape(in.Parent) + "/child/page?limit=100&expand=space,version,ancestors"
		if in.Parent == "" {
			if cf.space == "" {
				return nil, fmt.Errorf("set parent or plugins.confluence.space")
			}
			path = "/rest/api/space/" + url.PathEscape(cf.space) + "/content/page?depth=root&limit=100&expand=space,version,ancestors"
		}
		rows, err := cf.list(c, path)
		return frames.Of(rows), err
	}, sdk.Cache("60s"))
}
