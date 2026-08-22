package executor

import (
	"testing"
	"time"
)

func TestRequestProgressDefaultsToNil(t *testing.T) {
	var req Request
	if req.Progress != nil {
		t.Fatal("zero-value Request.Progress must be nil")
	}
}

func TestRequestProgressCarriesCanonicalEvent(t *testing.T) {
	progress := make(chan ProgressEvent, 1)
	req := Request{Progress: progress}
	want := ProgressEvent{Message: "running checks", At: time.Unix(42, 0)}

	req.Progress <- want
	if got := <-progress; got != want {
		t.Fatalf("progress event = %#v, want %#v", got, want)
	}
}
