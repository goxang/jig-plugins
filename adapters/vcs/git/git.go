// Package git is the vcs/1 plugin for the project's repository: status, log graph, branches, diff,
// blame, stashes and worktrees; commit, checkout, pull, push and the export of a ref.
package git

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/goxang/jig-plugins/internal/frames"
	"github.com/goxang/jig-plugins/internal/sh"
	"github.com/goxang/jig/sdk"
)

// Dir is the repository commands run in: the project jig opened the plugin for.
func Dir() string {
	if d := os.Getenv("JIG_PROJECT"); d != "" {
		return d
	}
	return "."
}

// Git runs git in the project and returns its trimmed output.
func Git(ctx context.Context, args ...string) (string, error) {
	c := sh.New("git", args...)
	c.Dir = Dir()
	out, err := c.Output(ctx)
	return strings.TrimRight(string(out), "\n"), err
}

// Remote is the forge host and project path of a remote's URL (ssh or https): gitlab.com, group/app.
func Remote(ctx context.Context, name string) (host, path string, err error) {
	u, err := Git(ctx, "remote", "get-url", name)
	if err != nil {
		return "", "", err
	}
	return ParseRemote(u)
}

func ParseRemote(u string) (host, path string, err error) {
	u = strings.TrimSuffix(strings.TrimSpace(u), ".git")
	if !strings.Contains(u, "://") {
		// scp-like: git@host:group/app
		if at := strings.Index(u, "@"); at >= 0 {
			u = u[at+1:]
		}
		h, p, ok := strings.Cut(u, ":")
		if !ok {
			return "", "", fmt.Errorf("cannot read remote %q", u)
		}
		return h, p, nil
	}
	pu, err := url.Parse(u)
	if err != nil {
		return "", "", err
	}
	return pu.Hostname(), strings.TrimPrefix(pu.Path, "/"), nil
}

type change struct {
	Path  string `json:"path"`
	State string `json:"state"`
}

type commit struct {
	Commit  string    `json:"commit"`
	Author  string    `json:"author"`
	Time    time.Time `json:"time"`
	Subject string    `json:"subject"`
	Refs    string    `json:"refs"`
	Graph   string    `json:"graph"`
}

type branch struct {
	Name     string    `json:"name"`
	Current  bool      `json:"current"`
	Upstream string    `json:"upstream"`
	Track    string    `json:"track"`
	Updated  time.Time `json:"updated"`
}

type blame struct {
	N      int       `json:"n"`
	Commit string    `json:"commit"`
	Author string    `json:"author"`
	Time   time.Time `json:"time"`
	Text   string    `json:"text"`
}

type stash struct {
	Ref     string `json:"ref"`
	Subject string `json:"subject"`
}

type worktree struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

var states = map[byte]string{'M': "modified", 'A': "added", 'D': "deleted", 'R': "renamed", 'C': "copied", 'U': "conflict", '?': "untracked", 'T': "type changed"}

// Status parses `git status --porcelain=v1`: a path's state, staged or not.
func Status(out string) []change {
	var cs []change
	for _, l := range strings.Split(out, "\n") {
		if len(l) < 4 {
			continue
		}
		x, y, path := l[0], l[1], l[3:]
		if _, to, ok := strings.Cut(path, " -> "); ok {
			path = to
		}
		state := states[y]
		if x != ' ' && x != '?' {
			state = "staged " + states[x]
			if y != ' ' {
				state += ", " + states[y]
			}
		}
		cs = append(cs, change{Path: strings.Trim(path, `"`), State: state})
	}
	return cs
}

const logFormat = "%x1f%H%x1f%an%x1f%at%x1f%s%x1f%D"

// Log parses `git log --graph --format=logFormat`: graph lines with no commit keep only their graph.
func Log(out string) []commit {
	var cs []commit
	for _, l := range strings.Split(out, "\n") {
		parts := strings.Split(l, "\x1f")
		c := commit{Graph: strings.TrimRight(parts[0], " ")}
		if len(parts) >= 6 {
			sec, _ := strconv.ParseInt(parts[3], 10, 64)
			c.Commit, c.Author, c.Time, c.Subject, c.Refs = parts[1][:min(12, len(parts[1]))], parts[2], time.Unix(sec, 0), parts[4], parts[5]
		}
		if c.Graph != "" || c.Commit != "" {
			cs = append(cs, c)
		}
	}
	return cs
}

// Blame parses `git blame --line-porcelain`.
func Blame(out string) []blame {
	var bs []blame
	var cur blame
	for _, l := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(l, "\t"):
			cur.Text = l[1:]
			cur.N = len(bs) + 1
			bs = append(bs, cur)
		case strings.HasPrefix(l, "author "):
			cur.Author = l[7:]
		case strings.HasPrefix(l, "author-time "):
			sec, _ := strconv.ParseInt(l[12:], 10, 64)
			cur.Time = time.Unix(sec, 0)
		default:
			if f := strings.Fields(l); len(f) >= 3 && len(f[0]) == 40 {
				cur.Commit = f[0][:12]
			}
		}
	}
	return bs
}

// Export checks a ref out into a cached directory and returns it; the core engine does it.
type Export func(ctx context.Context, ref string) (string, error)

func Register(p *sdk.Plugin, export Export) {
	p.Source("vcs.status", func(c *sdk.Ctx, _ sdk.Empty) (*sdk.Frame, error) {
		out, err := Git(c, "status", "--porcelain=v1", "--untracked-files=all")
		return frames.Of(Status(out)), err
	}, sdk.Describe("changed files: path and state"))

	p.Source("vcs.log", func(c *sdk.Ctx, in struct {
		Limit int    `json:"limit" desc:"commits (default 100)"`
		Path  string `json:"path" desc:"only commits touching this path"`
		Ref   string `json:"ref" desc:"branch or ref (default HEAD; all for every branch)"`
		Graph bool   `json:"graph" desc:"draw the branch graph"`
	}) (*sdk.Frame, error) {
		args := []string{"log", "--format=" + logFormat, "-n", strconv.Itoa(cmp.Or(in.Limit, 100))}
		if in.Graph {
			args = append(args, "--graph")
		}
		switch in.Ref {
		case "":
		case "all":
			args = append(args, "--all")
		default:
			args = append(args, in.Ref)
		}
		if in.Path != "" {
			args = append(args, "--", in.Path)
		}
		out, err := Git(c, args...)
		return frames.Of(Log(out)), err
	}, sdk.Describe("commits, newest first, with the graph on request"))

	p.Source("vcs.branches", func(c *sdk.Ctx, _ sdk.Empty) (*sdk.Frame, error) {
		out, err := Git(c, "for-each-ref", "--sort=-committerdate", "--format=%(HEAD)%1f%(refname:short)%1f%(upstream:short)%1f%(upstream:track)%1f%(committerdate:unix)", "refs/heads", "refs/remotes")
		var bs []branch
		for _, l := range strings.Split(out, "\n") {
			f := strings.Split(l, "\x1f")
			if len(f) < 5 || strings.HasSuffix(f[1], "/HEAD") {
				continue
			}
			sec, _ := strconv.ParseInt(f[4], 10, 64)
			bs = append(bs, branch{Name: f[1], Current: f[0] == "*", Upstream: f[2], Track: f[3], Updated: time.Unix(sec, 0)})
		}
		return frames.Of(bs), err
	}, sdk.Describe("local and remote branches, most recent first"))

	p.Source("vcs.diff", func(c *sdk.Ctx, in struct {
		Path   string `json:"path"`
		Ref    string `json:"ref" desc:"compare with this ref (default: the index); a..b for a range"`
		Staged bool   `json:"staged"`
	}) (*sdk.Frame, error) {
		args := []string{"diff", "--no-color"}
		if in.Staged {
			args = append(args, "--cached")
		}
		if in.Ref != "" {
			args = append(args, in.Ref)
		}
		if in.Path != "" {
			args = append(args, "--", in.Path)
		}
		out, err := Git(c, args...)
		return frames.Text(out), err
	}, sdk.Describe("a unified diff, a row per line"))

	p.Source("vcs.blame", func(c *sdk.Ctx, in struct {
		Path string `json:"path" jig:"required"`
		Ref  string `json:"ref"`
	}) (*sdk.Frame, error) {
		args := []string{"blame", "--line-porcelain"}
		if in.Ref != "" {
			args = append(args, in.Ref)
		}
		out, err := Git(c, append(args, "--", in.Path)...)
		return frames.Of(Blame(out)), err
	}, sdk.Describe("who last changed each line of a file"))

	p.Source("vcs.stashes", func(c *sdk.Ctx, _ sdk.Empty) (*sdk.Frame, error) {
		out, err := Git(c, "stash", "list", "--format=%gd%x1f%s")
		var ss []stash
		for _, l := range strings.Split(out, "\n") {
			if r, s, ok := strings.Cut(l, "\x1f"); ok {
				ss = append(ss, stash{r, s})
			}
		}
		return frames.Of(ss), err
	})

	p.Source("vcs.worktrees", func(c *sdk.Ctx, _ sdk.Empty) (*sdk.Frame, error) {
		out, err := Git(c, "worktree", "list", "--porcelain")
		var ws []worktree
		for _, block := range strings.Split(out, "\n\n") {
			var w worktree
			for _, l := range strings.Split(block, "\n") {
				k, v, _ := strings.Cut(l, " ")
				switch k {
				case "worktree":
					w.Path = v
				case "HEAD":
					w.Commit = v[:min(12, len(v))]
				case "branch":
					w.Branch = strings.TrimPrefix(v, "refs/heads/")
				}
			}
			if w.Path != "" {
				ws = append(ws, w)
			}
		}
		return frames.Of(ws), err
	})

	p.Action("vcs.checkout", sdk.EffectWrite, func(c *sdk.Ctx, in struct {
		Branch string `json:"branch" jig:"required"`
		Create bool   `json:"create" desc:"create the branch"`
	}) (*sdk.Result, error) {
		args := []string{"switch", in.Branch}
		if in.Create {
			args = []string{"switch", "-c", in.Branch}
		}
		return run(c, args...)
	}, sdk.Describe("switch to a branch"))

	p.Action("vcs.commit", sdk.EffectWrite, func(c *sdk.Ctx, in struct {
		Message string   `json:"message" jig:"required"`
		All     bool     `json:"all" desc:"stage every tracked change first"`
		Paths   []string `json:"paths" desc:"stage these first"`
	}) (*sdk.Result, error) {
		if len(in.Paths) > 0 {
			if _, err := Git(c, append([]string{"add", "--"}, in.Paths...)...); err != nil {
				return nil, err
			}
		}
		args := []string{"commit", "-m", in.Message}
		if in.All {
			args = append(args, "--all")
		}
		return run(c, args...)
	})

	p.Action("vcs.pull", sdk.EffectWrite, func(c *sdk.Ctx, _ sdk.Empty) (*sdk.Result, error) {
		return run(c, "pull", "--ff-only")
	}, sdk.Describe("fast-forward the current branch from its upstream"))

	p.Action("vcs.push", sdk.EffectDanger, func(c *sdk.Ctx, in struct {
		Remote string `json:"remote" desc:"default origin"`
		Branch string `json:"branch" desc:"default the current branch"`
		Force  bool   `json:"force" desc:"--force-with-lease"`
	}) (*sdk.Result, error) {
		args := []string{"push", "--set-upstream", cmp.Or(in.Remote, "origin"), cmp.Or(in.Branch, "HEAD")}
		if in.Force {
			args = append(args, "--force-with-lease")
		}
		return run(c, args...)
	}, sdk.Describe("push a branch (confirmed: it changes the shared remote)"))

	p.Action("vcs.stash", sdk.EffectWrite, func(c *sdk.Ctx, in struct {
		Pop     string `json:"pop" desc:"apply and drop this stash (stash@{0}) instead of saving one"`
		Message string `json:"message"`
	}) (*sdk.Result, error) {
		if in.Pop != "" {
			return run(c, "stash", "pop", in.Pop)
		}
		return run(c, "stash", "push", "--include-untracked", "-m", cmp.Or(in.Message, "jig"))
	})

	p.Action("vcs.export", sdk.EffectRead, func(c *sdk.Ctx, in struct {
		Ref string `json:"ref" jig:"required"`
	}) (*sdk.Result, error) {
		dir, err := export(c, in.Ref)
		if err != nil {
			return nil, err
		}
		return &sdk.Result{Message: dir, Outputs: sdk.Args{"dir": dir}}, nil
	}, sdk.Describe("check a ref out into a cached directory (built, run or tested without touching the working tree)"))
}

func run(c *sdk.Ctx, args ...string) (*sdk.Result, error) {
	out, err := Git(c, args...)
	for _, l := range strings.Split(out, "\n") {
		if l != "" {
			c.Line(l)
		}
	}
	if err != nil {
		return nil, err
	}
	return &sdk.Result{Message: "git " + args[0] + " done"}, nil
}
