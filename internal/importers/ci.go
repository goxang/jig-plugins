package importers

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// gitlabKeywords are .gitlab-ci.yml's top-level keys that are not jobs.
var gitlabKeywords = []string{"default", "include", "stages", "variables", "workflow", "image", "services", "cache", "before_script", "after_script"}

type gitlabJob struct {
	Extends      any               `yaml:"extends"`
	Stage        string            `yaml:"stage"`
	Variables    map[string]any    `yaml:"variables"`
	BeforeScript any               `yaml:"before_script"`
	Script       any               `yaml:"script"`
	Trigger      any               `yaml:"trigger"`
	vars         map[string]string `yaml:"-"`
}

// GitLabCI turns the jobs of a .gitlab-ci.yml into tasks: before_script and script with the job's
// variables, extends resolved. Trigger jobs and jobs with no script are left out; included files are not read.
func GitLabCI(path, _ string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	jobs := map[string]*gitlabJob{}
	for name, n := range doc {
		if slices.Contains(gitlabKeywords, name) || n.Kind != yaml.MappingNode {
			continue
		}
		var j gitlabJob
		if err := n.Decode(&j); err != nil {
			return nil, fmt.Errorf("job %s: %w", name, err)
		}
		jobs[name] = &j
	}
	var global struct {
		Variables    map[string]any `yaml:"variables"`
		BeforeScript any            `yaml:"before_script"`
		Default      struct {
			BeforeScript any `yaml:"before_script"`
		} `yaml:"default"`
	}
	_ = yaml.Unmarshal(raw, &global)
	globalBefore := global.Default.BeforeScript
	if globalBefore == nil {
		globalBefore = global.BeforeScript
	}

	var resolve func(name string, depth int) *gitlabJob
	resolve = func(name string, depth int) *gitlabJob {
		j := jobs[name]
		if j == nil || depth > 10 {
			return &gitlabJob{}
		}
		out := &gitlabJob{vars: map[string]string{}}
		for _, parent := range lines(j.Extends) {
			p := resolve(parent, depth+1)
			for k, v := range p.vars {
				out.vars[k] = v
			}
			out.Stage, out.BeforeScript, out.Script, out.Trigger = or(p.Stage, out.Stage), orAny(p.BeforeScript, out.BeforeScript), orAny(p.Script, out.Script), orAny(p.Trigger, out.Trigger)
		}
		for k, v := range j.Variables {
			if m, ok := v.(map[string]any); ok {
				v = m["value"]
			}
			out.vars[k] = fmt.Sprint(v)
		}
		out.Stage, out.BeforeScript, out.Script, out.Trigger = or(j.Stage, out.Stage), orAny(j.BeforeScript, out.BeforeScript), orAny(j.Script, out.Script), orAny(j.Trigger, out.Trigger)
		return out
	}

	out := map[string]task{}
	for name := range jobs {
		if strings.HasPrefix(name, ".") {
			continue
		}
		j := resolve(name, 0)
		if j.Trigger != nil || j.Script == nil {
			continue
		}
		before := j.BeforeScript
		if before == nil {
			before = globalBefore
		}
		vars := map[string]string{}
		for k, v := range global.Variables {
			if m, ok := v.(map[string]any); ok {
				v = m["value"]
			}
			vars[k] = fmt.Sprint(v)
		}
		for k, v := range j.vars {
			vars[k] = v
		}
		steps := exports(vars)
		for _, l := range append(lines(before), lines(j.Script)...) {
			steps = append(steps, shell(l))
		}
		out["ci-"+name] = task{Help: "GitLab CI job " + name + stageNote(j.Stage), Group: "ci", Steps: []string{strings.Join(steps, "\n")}}
	}
	return tasks(out), nil
}

// GitHubActions turns the jobs of .github/workflows into tasks from their run: steps; uses: steps are
// left out (named in the task's help), and a job with no run: step is skipped.
func GitHubActions(path, _ string) (map[string]any, error) {
	files, err := yamlFiles(path, ".yml", ".yaml")
	if err != nil {
		return nil, err
	}
	out := map[string]task{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var wf struct {
			Name string            `yaml:"name"`
			Env  map[string]string `yaml:"env"`
			Jobs map[string]struct {
				Name  string            `yaml:"name"`
				Env   map[string]string `yaml:"env"`
				Steps []struct {
					Name string            `yaml:"name"`
					Uses string            `yaml:"uses"`
					Run  string            `yaml:"run"`
					Env  map[string]string `yaml:"env"`
					Dir  string            `yaml:"working-directory"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(raw, &wf); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		workflow := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		for id, job := range wf.Jobs {
			var steps, skipped []string
			for _, s := range job.Steps {
				if s.Run == "" {
					if s.Uses != "" {
						skipped = append(skipped, s.Uses)
					}
					continue
				}
				if strings.Contains(s.Run, "${{") {
					skipped = append(skipped, or(s.Name, "a step with ${{ }} expressions"))
					continue
				}
				vars := map[string]string{}
				for _, m := range []map[string]string{wf.Env, job.Env, s.Env} {
					for k, v := range m {
						if !strings.Contains(v, "${{") {
							vars[k] = v
						}
					}
				}
				step := strings.Join(append(exports(vars), shell(strings.TrimSpace(s.Run))), "\n")
				if s.Dir != "" {
					step = "cd " + s.Dir + "\n" + step
				}
				if s.Name != "" {
					step = "# " + s.Name + "\n" + step
				}
				steps = append(steps, step)
			}
			if len(steps) == 0 {
				continue
			}
			help := "GitHub Actions " + or(wf.Name, workflow) + " › " + or(job.Name, id)
			if len(skipped) > 0 {
				help += " (skips " + strings.Join(skipped, ", ") + ")"
			}
			out["gh-"+workflow+"-"+id] = task{Help: help, Group: "ci", Steps: steps}
		}
	}
	return tasks(out), nil
}

func exports(vars map[string]string) []string {
	var out []string
	for _, k := range sortedKeys(vars) {
		out = append(out, "export "+k+"="+quote(shell(vars[k])))
	}
	return out
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// lines reads a CI script: one string or a list of strings (nested lists flattened, as GitLab allows).
func lines(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, e := range v {
			out = append(out, lines(e)...)
		}
		return out
	}
	return nil
}

func stageNote(s string) string {
	if s == "" {
		return ""
	}
	return " (stage " + s + ")"
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func orAny(a, b any) any {
	if a != nil {
		return a
	}
	return b
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
