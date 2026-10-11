package forge

import (
	"context"
	"strings"
	"time"

	"github.com/goxang/jig-plugins/internal/frames"
	"github.com/goxang/jig/sdk"
)

type MR struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Author  string    `json:"author"`
	State   string    `json:"state"`
	Source  string    `json:"source"`
	Target  string    `json:"target"`
	Updated time.Time `json:"updated"`
	URL     string    `json:"url"`
}

type File struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	Patch   string `json:"-"`
	Changed int    `json:"changed"`
}

type Pipeline struct {
	ID       string        `json:"id"`
	Ref      string        `json:"ref"`
	Status   string        `json:"status"`
	Started  time.Time     `json:"started"`
	Duration time.Duration `json:"duration"`
	URL      string        `json:"url"`
}

type Job struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Stage    string        `json:"stage"`
	Status   string        `json:"status"`
	Duration time.Duration `json:"duration"`
}

type Release struct {
	Tag      string    `json:"tag"`
	Name     string    `json:"name"`
	Released time.Time `json:"released"`
	URL      string    `json:"url"`
}

type Issue struct {
	Key      string    `json:"key"`
	Title    string    `json:"title"`
	Status   string    `json:"status"`
	Assignee string    `json:"assignee"`
	Priority string    `json:"priority"`
	Updated  time.Time `json:"updated"`
	URL      string    `json:"url"`
}

type Comment struct {
	Author string    `json:"author"`
	Time   time.Time `json:"time"`
	Body   string    `json:"body"`
}

type IssueDetail struct {
	Key         string    `json:"key"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	Comments    []Comment `json:"comments"`
}

// SCM is a hosted forge as scm/1 and tracker/1 see it; gitlab and github implement it.
type SCM interface {
	MRs(ctx context.Context, state, author string) ([]MR, error)
	Files(ctx context.Context, mr string) ([]File, error)
	Approve(ctx context.Context, mr string) error
	Merge(ctx context.Context, mr string) error
	Comment(ctx context.Context, mr, body string) error
	Pipelines(ctx context.Context, ref string, limit int) ([]Pipeline, error)
	Jobs(ctx context.Context, pipeline string) ([]Job, error)
	// Log is a job's output so far and whether the job has finished.
	Log(ctx context.Context, job string) ([]string, bool, error)
	Retry(ctx context.Context, job string) error
	Releases(ctx context.Context) ([]Release, error)
	Issues(ctx context.Context, query, assignee string) ([]Issue, error)
	Issue(ctx context.Context, key string) (*IssueDetail, error)
	Transition(ctx context.Context, key, status string) error
	IssueComment(ctx context.Context, key, body string) error
}

type service struct {
	Service string `json:"service"`
}

// RegisterSCM serves scm/1 and tracker/1 from a forge opened per call.
func RegisterSCM(p *sdk.Plugin, open func(*sdk.Ctx) (SCM, error), affected Affected) {
	type mrArg struct {
		MR string `json:"mr" jig:"required"`
	}
	p.Source("scm.mrs", func(c *sdk.Ctx, in struct {
		State  string `json:"state" desc:"opened (default), merged, closed or all"`
		Author string `json:"author"`
	}) (*sdk.Frame, error) {
		f, err := open(c)
		if err != nil {
			return nil, err
		}
		rows, err := f.MRs(c, in.State, in.Author)
		return frames.Of(rows), err
	}, sdk.Cache("30s"), sdk.Describe("merge requests (pull requests)"))

	p.Source("scm.files", func(c *sdk.Ctx, in mrArg) (*sdk.Frame, error) {
		f, err := open(c)
		if err != nil {
			return nil, err
		}
		rows, err := f.Files(c, in.MR)
		return frames.Of(rows), err
	}, sdk.Describe("the files a merge request changes"))

	p.Source("scm.diff", func(c *sdk.Ctx, in mrArg) (*sdk.Frame, error) {
		f, err := open(c)
		if err != nil {
			return nil, err
		}
		files, err := f.Files(c, in.MR)
		type line struct {
			Line string `json:"line"`
		}
		var ls []line
		for _, fl := range files {
			ls = append(ls, line{"diff " + fl.Path})
			for _, l := range strings.Split(fl.Patch, "\n") {
				ls = append(ls, line{l})
			}
		}
		return frames.Of(ls), err
	}, sdk.Describe("a merge request's diff, a row per line"))

	p.Source("scm.services", func(c *sdk.Ctx, in mrArg) (*sdk.Frame, error) {
		f, err := open(c)
		if err != nil {
			return nil, err
		}
		files, err := f.Files(c, in.MR)
		if err != nil {
			return nil, err
		}
		paths := make([]string, len(files))
		for i, fl := range files {
			paths[i] = fl.Path
		}
		svcs, err := affected(c, paths)
		var rows []service
		for _, s := range svcs {
			rows = append(rows, service{s})
		}
		return frames.Of(rows), err
	}, sdk.Describe("the project's services a merge request touches: what to build, deploy and test"))

	p.Action("scm.approve", sdk.EffectWrite, func(c *sdk.Ctx, in mrArg) (*sdk.Result, error) {
		return do(c, open, "approved "+in.MR, func(f SCM) error { return f.Approve(c, in.MR) })
	})
	p.Action("scm.merge", sdk.EffectDanger, func(c *sdk.Ctx, in mrArg) (*sdk.Result, error) {
		return do(c, open, "merged "+in.MR, func(f SCM) error { return f.Merge(c, in.MR) })
	})
	p.Action("scm.comment", sdk.EffectWrite, func(c *sdk.Ctx, in struct {
		MR   string `json:"mr" jig:"required"`
		Body string `json:"body" jig:"required"`
	}) (*sdk.Result, error) {
		return do(c, open, "commented on "+in.MR, func(f SCM) error { return f.Comment(c, in.MR, in.Body) })
	})

	p.Source("scm.pipelines", func(c *sdk.Ctx, in struct {
		Ref   string `json:"ref"`
		Limit int    `json:"limit"`
	}) (*sdk.Frame, error) {
		f, err := open(c)
		if err != nil {
			return nil, err
		}
		if in.Limit <= 0 {
			in.Limit = 30
		}
		rows, err := f.Pipelines(c, in.Ref, in.Limit)
		return frames.Of(rows), err
	}, sdk.Cache("15s"))

	p.Source("scm.jobs", func(c *sdk.Ctx, in struct {
		Pipeline string `json:"pipeline" jig:"required"`
	}) (*sdk.Frame, error) {
		f, err := open(c)
		if err != nil {
			return nil, err
		}
		rows, err := f.Jobs(c, in.Pipeline)
		return frames.Of(rows), err
	})

	p.Stream("scm.log", func(c *sdk.Ctx, in struct {
		Job    string `json:"job" jig:"required"`
		Follow bool   `json:"follow" desc:"keep reading until the job ends"`
	}, out *sdk.Out) error {
		f, err := open(c)
		if err != nil {
			return err
		}
		sent := 0
		for {
			ls, done, err := f.Log(c, in.Job)
			if err != nil {
				return err
			}
			for _, l := range ls[min(sent, len(ls)):] {
				if err := out.Line(l); err != nil {
					return err
				}
			}
			sent = max(sent, len(ls))
			if done || !in.Follow {
				return nil
			}
			select {
			case <-c.Done():
				return nil
			case <-time.After(3 * time.Second):
			}
		}
	}, sdk.Describe("a CI job's log, followed while it runs"))

	p.Action("scm.retry", sdk.EffectWrite, func(c *sdk.Ctx, in struct {
		Job string `json:"job" jig:"required"`
	}) (*sdk.Result, error) {
		return do(c, open, "retried job "+in.Job, func(f SCM) error { return f.Retry(c, in.Job) })
	})

	p.Source("scm.releases", func(c *sdk.Ctx, _ sdk.Empty) (*sdk.Frame, error) {
		f, err := open(c)
		if err != nil {
			return nil, err
		}
		rows, err := f.Releases(c)
		return frames.Of(rows), err
	}, sdk.Cache("60s"))

	p.Source("tracker.issues", func(c *sdk.Ctx, in struct {
		Query    string `json:"query" desc:"search text"`
		Assignee string `json:"assignee"`
	}) (*sdk.Frame, error) {
		f, err := open(c)
		if err != nil {
			return nil, err
		}
		rows, err := f.Issues(c, in.Query, in.Assignee)
		return frames.Of(rows), err
	}, sdk.Cache("30s"))

	p.Source("tracker.issue", func(c *sdk.Ctx, in struct {
		Key string `json:"key" jig:"required"`
	}) (*sdk.Frame, error) {
		f, err := open(c)
		if err != nil {
			return nil, err
		}
		d, err := f.Issue(c, in.Key)
		if err != nil {
			return nil, err
		}
		return frames.Of([]IssueDetail{*d}), nil
	})

	p.Action("tracker.transition", sdk.EffectWrite, func(c *sdk.Ctx, in struct {
		Key    string `json:"key" jig:"required"`
		Status string `json:"status" jig:"required" desc:"closed or open"`
	}) (*sdk.Result, error) {
		return do(c, open, in.Key+" "+in.Status, func(f SCM) error { return f.Transition(c, in.Key, in.Status) })
	})

	p.Action("tracker.comment", sdk.EffectWrite, func(c *sdk.Ctx, in struct {
		Key  string `json:"key" jig:"required"`
		Body string `json:"body" jig:"required"`
	}) (*sdk.Result, error) {
		return do(c, open, "commented on "+in.Key, func(f SCM) error { return f.IssueComment(c, in.Key, in.Body) })
	})
}

func do(c *sdk.Ctx, open func(*sdk.Ctx) (SCM, error), msg string, f func(SCM) error) (*sdk.Result, error) {
	s, err := open(c)
	if err != nil {
		return nil, err
	}
	if err := f(s); err != nil {
		return nil, err
	}
	return &sdk.Result{Message: msg}, nil
}
