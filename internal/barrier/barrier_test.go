package barrier

import (
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

func sortedCopy(ss []string) []string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	return out
}

func assertStringSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	gotSorted, wantSorted := sortedCopy(got), sortedCopy(want)
	if len(gotSorted) != len(wantSorted) {
		t.Fatalf("%s = %v, want %v", label, gotSorted, wantSorted)
	}
	for i := range gotSorted {
		if gotSorted[i] != wantSorted[i] {
			t.Fatalf("%s = %v, want %v", label, gotSorted, wantSorted)
		}
	}
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name          string
		expected      []string
		observed      []lane.State
		wantReleased  bool
		wantIntegrate []string
		wantPreserve  []string
	}{
		{
			name:     "all done releases",
			expected: []string{"a", "b", "c"},
			observed: []lane.State{
				{LaneID: "a", Status: lane.Done},
				{LaneID: "b", Status: lane.Done},
				{LaneID: "c", Status: lane.Done},
			},
			wantReleased:  true,
			wantIntegrate: []string{"a", "b", "c"},
			wantPreserve:  []string{},
		},
		{
			name:     "one done rest running does not release",
			expected: []string{"a", "b", "c"},
			observed: []lane.State{
				{LaneID: "a", Status: lane.Done},
				{LaneID: "b", Status: lane.Pending},
				{LaneID: "c", Status: lane.Running},
			},
			wantReleased: false,
		},
		{
			name:     "lone blocked lane with rest done releases",
			expected: []string{"a", "b", "c"},
			observed: []lane.State{
				{LaneID: "a", Status: lane.Blocked},
				{LaneID: "b", Status: lane.Done},
				{LaneID: "c", Status: lane.Done},
			},
			wantReleased:  true,
			wantIntegrate: []string{"b", "c"},
			wantPreserve:  []string{"a"},
		},
		{
			name:     "lone blocked lane does not release early while a peer still running",
			expected: []string{"a", "b"},
			observed: []lane.State{
				{LaneID: "a", Status: lane.Blocked},
				{LaneID: "b", Status: lane.Running},
			},
			wantReleased: false,
		},
		{
			name:     "all blocked releases with zero eligible",
			expected: []string{"a", "b", "c"},
			observed: []lane.State{
				{LaneID: "a", Status: lane.Blocked},
				{LaneID: "b", Status: lane.Blocked},
				{LaneID: "c", Status: lane.Blocked},
			},
			wantReleased:  true,
			wantIntegrate: []string{},
			wantPreserve:  []string{"a", "b", "c"},
		},
		{
			name:     "single-lane batch releases on its own terminal state",
			expected: []string{"only"},
			observed: []lane.State{
				{LaneID: "only", Status: lane.Done},
			},
			wantReleased:  true,
			wantIntegrate: []string{"only"},
			wantPreserve:  []string{},
		},
		{
			name:     "mixed two done one blocked splits eligible and preserved",
			expected: []string{"a", "b", "c"},
			observed: []lane.State{
				{LaneID: "a", Status: lane.Done},
				{LaneID: "b", Status: lane.Done},
				{LaneID: "c", Status: lane.Blocked},
			},
			wantReleased:  true,
			wantIntegrate: []string{"a", "b"},
			wantPreserve:  []string{"c"},
		},
		{
			name:     "all non-done terminal set has zero eligible and all preserved",
			expected: []string{"a", "b", "c"},
			observed: []lane.State{
				{LaneID: "a", Status: lane.Blocked},
				{LaneID: "b", Status: lane.Deviated},
				{LaneID: "c", Status: lane.Failed},
			},
			wantReleased:  true,
			wantIntegrate: []string{},
			wantPreserve:  []string{"a", "b", "c"},
		},
		{
			name:     "no state supplied for one lane means no release and no timeout mutation",
			expected: []string{"a", "b"},
			observed: []lane.State{
				{LaneID: "a", Status: lane.Done},
				// "b" never observed at all.
			},
			wantReleased: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.expected, tt.observed)
			if got.Released != tt.wantReleased {
				t.Fatalf("Released = %v, want %v", got.Released, tt.wantReleased)
			}
			if tt.wantReleased {
				assertStringSet(t, "Integrate", got.Integrate, tt.wantIntegrate)
				assertStringSet(t, "Preserve", got.Preserve, tt.wantPreserve)
			}
		})
	}
}

func TestNewRejectsEmptyOrDuplicateLaneIDs(t *testing.T) {
	tests := []struct {
		name    string
		laneIDs []string
	}{
		{"empty lane list", []string{}},
		{"nil lane list", nil},
		{"duplicate lane id", []string{"a", "b", "a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.laneIDs); err == nil {
				t.Fatalf("New(%v) = nil error, want an error", tt.laneIDs)
			}
		})
	}
}

func TestNewAcceptsUniqueNonEmptyLaneIDs(t *testing.T) {
	if _, err := New([]string{"a", "b", "c"}); err != nil {
		t.Fatalf("New with unique lane ids returned error: %v", err)
	}
}

func TestObserveUnexpectedLaneErrors(t *testing.T) {
	b, err := New([]string{"a", "b"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := b.Observe(lane.State{LaneID: "c", Status: lane.Done}); err == nil {
		t.Fatal("Observe(unexpected lane) = nil error, want an error")
	}
}

func TestDoneClosesExactlyOnceOnFullRelease(t *testing.T) {
	b, err := New([]string{"a", "b"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	select {
	case <-b.Done():
		t.Fatal("Done() closed before any lane observed")
	default:
	}

	if err := b.Observe(lane.State{LaneID: "a", Status: lane.Done}); err != nil {
		t.Fatalf("Observe(a): %v", err)
	}

	select {
	case <-b.Done():
		t.Fatal("Done() closed with lane b still non-terminal")
	default:
	}

	if err := b.Observe(lane.State{LaneID: "b", Status: lane.Blocked}); err != nil {
		t.Fatalf("Observe(b): %v", err)
	}

	select {
	case <-b.Done():
	default:
		t.Fatal("Done() not closed after every lane reached a terminal state")
	}
}

func TestReObserveAfterReleaseDoesNotRefireOrMutateOutcome(t *testing.T) {
	b, err := New([]string{"a", "b"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := b.Observe(lane.State{LaneID: "a", Status: lane.Done}); err != nil {
		t.Fatalf("Observe(a): %v", err)
	}
	if err := b.Observe(lane.State{LaneID: "b", Status: lane.Done}); err != nil {
		t.Fatalf("Observe(b): %v", err)
	}

	before := b.Outcome()
	assertStringSet(t, "Integrate", before.Integrate, []string{"a", "b"})

	// A late/duplicate update after release must not panic (double-close)
	// and must not change the already-computed outcome.
	if err := b.Observe(lane.State{LaneID: "a", Status: lane.Blocked}); err != nil {
		t.Fatalf("Observe(a) post-release: %v", err)
	}

	select {
	case <-b.Done():
	default:
		t.Fatal("Done() unexpectedly not closed after release")
	}

	after := b.Outcome()
	assertStringSet(t, "Integrate", after.Integrate, []string{"a", "b"})
	assertStringSet(t, "Preserve", after.Preserve, []string{})
}

func TestObserveConcurrentIsRaceFree(t *testing.T) {
	const n = 16
	laneIDs := make([]string, n)
	for i := 0; i < n; i++ {
		laneIDs[i] = fmt.Sprintf("lane-%d", i)
	}

	b, err := New(laneIDs)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		id := laneIDs[i]
		go func() {
			defer wg.Done()
			if err := b.Observe(lane.State{LaneID: id, Status: lane.Done}); err != nil {
				t.Errorf("Observe(%s): %v", id, err)
			}
		}()
	}
	wg.Wait()

	select {
	case <-b.Done():
	default:
		t.Fatal("Done() not closed after all lanes observed concurrently")
	}

	assertStringSet(t, "Integrate", b.Outcome().Integrate, laneIDs)
}
