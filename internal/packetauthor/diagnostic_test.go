package packetauthor_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/packetauthor"
)

func TestAdmitBatchReturnsStableOrderedDeduplicatedDiagnostics(t *testing.T) {
	badBinding := validFeatureBinding()
	badBinding.Feature.LiveParentSHA = sha('c')
	items := []packetauthor.BatchItem{
		{Manual: &packetauthor.ManualPacket{Body: []byte("## Done criteria\n- x\n## Return\n"), ReadOnly: true, Binding: badBinding}},
		{Contract: func() *packetauthor.Contract {
			c := validContract()
			c.RouteIntent = ""
			c.WritePaths = []string{"../escape", "../escape"}
			return &c
		}(), Binding: packetauthor.TargetBinding{}},
	}
	_, err := packetauthor.AdmitBatch(items)
	var diagnostics packetauthor.Diagnostics
	if !errors.As(err, &diagnostics) {
		t.Fatalf("AdmitBatch() error = %T %v, want Diagnostics", err, err)
	}
	want := []string{
		"0:20:body:-1:PA_RESULT_PATH_MISSING",
		"0:30:body:-1:PA_RESULT_SCHEMA_MISSING",
		"0:40:route_intent:-1:PA_ROUTE_INVALID",
		"0:80:expected_parent_sha:-1:PA_TARGET_STALE",
		"1:40:route_intent:-1:PA_ROUTE_INVALID",
		"1:70:target:-1:PA_TARGET_INCOMPLETE",
		"1:90:write_paths:0:PA_PATH_INVALID",
		"1:90:write_paths:1:PA_PATH_INVALID",
	}
	got := make([]string, len(diagnostics))
	for i, diagnostic := range diagnostics {
		got[i] = diagnostic.Key()
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostic order\ngot:  %#v\nwant: %#v", got, want)
	}
}

func assertDiagnosticCode(t *testing.T, err error, want string) {
	t.Helper()
	var diagnostics packetauthor.Diagnostics
	if !errors.As(err, &diagnostics) {
		t.Fatalf("error = %T %v, want Diagnostics containing %s", err, err, want)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == want {
			return
		}
	}
	t.Fatalf("diagnostics = %#v, want code %s", diagnostics, want)
}
