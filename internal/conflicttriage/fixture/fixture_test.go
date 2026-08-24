package fixture_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/conflicttriage/fixture"
	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/run"
)

func openFixtureLedger(t *testing.T) *ledger.Ledger {
	t.Helper()
	l, err := ledger.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func TestFixtureGenerator_ForcesClassRequired(t *testing.T) {
	ctx := context.Background()
	l := openFixtureLedger(t)
	repo := t.TempDir()

	fix, err := fixture.GenerateFixture(ctx, fixture.GeneratorOptions{
		RepoRoot:   repo,
		Ledger:     l,
		FeatureAID: "feat-conflict-a",
		FeatureBID: "feat-conflict-b",
		ParentRefA: "refs/heads/feature-conflict-a",
		ParentRefB: "refs/heads/feature-conflict-b",
		SharedBase: true,
	})
	if err != nil {
		t.Fatalf("GenerateFixture: %v", err)
	}
	if fix.BaseSHA == "" {
		t.Fatal("BaseSHA empty, want shared registered base")
	}
	if fix.FeatureA.BaseSHA != fix.BaseSHA || fix.FeatureB.BaseSHA != fix.BaseSHA {
		t.Fatalf("features do not share base_sha: a=%q b=%q base=%q", fix.FeatureA.BaseSHA, fix.FeatureB.BaseSHA, fix.BaseSHA)
	}
	if fix.FeatureASHA == "" || fix.FeatureBSHA == "" {
		t.Fatalf("feature SHAs empty: a=%q b=%q", fix.FeatureASHA, fix.FeatureBSHA)
	}

	ev, err := overlap.Evaluate(ctx, fix.RepoRoot, fix.BaseSHA, fix.FeatureASHA, fix.FeatureBSHA)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	class, _ := overlap.Classify(ev.Signals, overlap.DefaultThresholds())
	if class != overlap.ClassRequired {
		t.Fatalf("Classify = %q, want %q (fixture must force ClassRequired through existing thresholds)", class, overlap.ClassRequired)
	}
	if ev.Class != overlap.ClassRequired {
		t.Errorf("Evaluate Class = %q, want %q", ev.Class, overlap.ClassRequired)
	}

	toy := filepath.Join(fix.RepoRoot, fix.ToyPath)
	if _, err := os.Stat(toy); err != nil {
		t.Fatalf("toy file %s: %v", toy, err)
	}
	var hunkCount int
	for _, ha := range ev.Signals.HunkAnalyses {
		if ha.Path == fix.ToyPath || filepath.Base(ha.Path) == filepath.Base(fix.ToyPath) {
			if n := len(ha.HunksA); n > hunkCount {
				hunkCount = n
			}
			if n := len(ha.HunksB); n > hunkCount {
				hunkCount = n
			}
		}
	}
	if hunkCount != 3 {
		t.Errorf("toy hunk count = %d, want 3 (business, slice-literal union, rename-vs-edit); analyses=%+v", hunkCount, ev.Signals.HunkAnalyses)
	}
}

func TestFixtureGenerator_MissingBaseSHASkipsClassRequired(t *testing.T) {
	ctx := context.Background()
	l := openFixtureLedger(t)
	repo := t.TempDir()

	fix, err := fixture.GenerateFixture(ctx, fixture.GeneratorOptions{
		RepoRoot:   repo,
		Ledger:     l,
		FeatureAID: "feat-divergent-a",
		FeatureBID: "feat-divergent-b",
		ParentRefA: "refs/heads/feature-divergent-a",
		ParentRefB: "refs/heads/feature-divergent-b",
		SharedBase: false,
	})
	if err != nil {
		t.Fatalf("GenerateFixture(divergent): %v", err)
	}
	if fix.FeatureA.BaseSHA == "" || fix.FeatureB.BaseSHA == "" {
		t.Fatal("feature.Create requires non-empty base_sha; divergent mode still registers SHAs")
	}
	if fix.FeatureA.BaseSHA == fix.FeatureB.BaseSHA {
		t.Fatalf("divergent features unexpectedly share base_sha %q", fix.FeatureA.BaseSHA)
	}

	// Gate passes an empty common base when registered base_shas diverge.
	ev, err := overlap.Evaluate(ctx, fix.RepoRoot, "", fix.FeatureASHA, fix.FeatureBSHA)
	if err == nil {
		t.Fatalf("Evaluate with missing common base succeeded with class %q, want ErrNoMergeBase (must not yield ClassRequired)", ev.Class)
	}
	if !errors.Is(err, overlap.ErrNoMergeBase) {
		t.Fatalf("Evaluate error = %v, want ErrNoMergeBase", err)
	}
}

func TestFixturePackets_DisjointAndValidParentRef(t *testing.T) {
	entries, err := os.ReadDir("packets")
	if err != nil {
		t.Fatalf("ReadDir(packets): %v", err)
	}
	var packets []packet.Packet
	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		names = append(names, e.Name())
		raw, err := os.ReadFile(filepath.Join("packets", e.Name()))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", e.Name(), err)
		}
		p, err := packet.Parse(strings.NewReader(string(raw)))
		if err != nil {
			t.Fatalf("Parse(%s): %v", e.Name(), err)
		}
		packets = append(packets, p)
		if err := feature.ValidateParentRef(p.ParentRef); err != nil {
			t.Errorf("%s parent_ref %q: %v", e.Name(), p.ParentRef, err)
		}
		if _, _, err := run.FeatureTarget([]packet.Packet{p}); err != nil {
			t.Errorf("FeatureTarget(%s): %v", e.Name(), err)
		}
	}
	if !contains(names, "claude_judge.md") || !contains(names, "opencode_judge.md") {
		t.Errorf("packets = %v, want claude_judge.md and opencode_judge.md", names)
	}
	if err := packet.DisjointAllowedPaths(packets); err != nil {
		t.Errorf("DisjointAllowedPaths: %v", err)
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func parsePacketFile(t *testing.T, name string) (packet.Packet, string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("packets", name))
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", name, err)
	}
	p, err := packet.Parse(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return p, string(raw)
}

// TestFixturePackets_AreBuildScopeTemplatesNotToyWriters locks the reading
// the tree already settled: feat_a/feat_b are sequential disjoint build-scope
// templates. GenerateFixture writes ToyPath via git; these packets must not
// grant it, and a reader must hit that fact before assuming allowed_paths.
func TestFixturePackets_AreBuildScopeTemplatesNotToyWriters(t *testing.T) {
	for _, name := range []string{"feat_a.md", "feat_b.md"} {
		p, raw := parsePacketFile(t, name)
		if packet.PathInScope(fixture.ToyPath, p.AllowedPaths) {
			t.Errorf("%s allowed_paths %v cover ToyPath %q; build-scope templates must not grant the toy", name, p.AllowedPaths, fixture.ToyPath)
		}
		lines := strings.Split(raw, "\n")
		if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
			t.Fatalf("%s: want --- frontmatter opener, got %q", name, raw)
		}
		if !strings.HasPrefix(strings.TrimSpace(lines[1]), "#") {
			t.Errorf("%s: want a # comment before frontmatter keys so a reader hits it before assuming allowed_paths, got %q", name, lines[1])
		}
		if !strings.Contains(lines[1], fixture.ToyPath) {
			t.Errorf("%s leading comment %q must name %s before allowed_paths is assumed", name, lines[1], fixture.ToyPath)
		}
		if !strings.Contains(p.Body, "GenerateFixture") {
			t.Errorf("%s body must say GenerateFixture writes the toy independently, got %q", name, p.Body)
		}
		if strings.Contains(p.RoutedBy, "of the three-hunk overlap toy") {
			t.Errorf("%s routed_by %q still describes these packets as the toy's dispatch shape", name, p.RoutedBy)
		}
	}
}
