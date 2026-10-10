package main

import (
	"testing"

	"github.com/goxang/jig/contracts"
	"github.com/goxang/jig/plugin"
	"github.com/goxang/jig/protocol"
)

func TestEveryPluginServesItsContracts(t *testing.T) {
	registered := map[string]bool{}
	for _, a := range plugin.List() {
		registered[string(a.Kind)+"/"+a.Type] = true
	}
	for _, n := range names() {
		for _, k := range plugins[n] {
			if !registered[string(k.kind)+"/"+n] {
				t.Errorf("%s: no %s adapter registered as %q", n, k.kind, n)
			}
			s, err := contracts.Serve(k.kind, k.probe, contracts.Lookup{})
			if err != nil {
				t.Fatal(err)
			}
			pr := &protocol.Provides{Sources: map[string]protocol.SourceSpec{}, Actions: map[string]protocol.ActionSpec{}}
			for x := range s.Sources {
				pr.Sources[x] = protocol.SourceSpec{}
			}
			for x := range s.Streams {
				pr.Sources[x] = protocol.SourceSpec{Stream: true}
			}
			for x := range s.Actions {
				pr.Actions[x] = protocol.ActionSpec{Effect: s.Effects[x]}
			}
			ct := contracts.ForKind(string(k.kind))
			if ps := ct.Check(pr); len(ps) > 0 {
				t.Errorf("%s as %s: %v", n, ct.ID, ps)
			}
		}
	}
}
