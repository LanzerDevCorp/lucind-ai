// Package packetauthor compiles and admits packet authoring contracts.
package packetauthor

import "fmt"

const (
	ContractVersion = "packet-author/v1"
	ManifestVersion = "packet-manifest/v1"
	LegacyVersion   = "legacy/v1"
)
const (
	CodeContractInvalid     = "PA_CONTRACT_INVALID"
	CodeManualMarkerInvalid = "PA_MANUAL_MARKER_INVALID"
	CodeResultPathMissing   = "PA_RESULT_PATH_MISSING"
	CodeResultSchemaMissing = "PA_RESULT_SCHEMA_MISSING"
	CodeRouteInvalid        = "PA_ROUTE_INVALID"
	CodeModeCommitConflict  = "PA_MODE_COMMIT_CONFLICT"
	CodeForbiddenTarget     = "PA_FORBIDDEN_TARGET"
	CodeTargetIncomplete    = "PA_TARGET_INCOMPLETE"
	CodeTargetStale         = "PA_TARGET_STALE"
	CodePathInvalid         = "PA_PATH_INVALID"
)

type Mode string

const (
	ModeWrite    Mode = "write"
	ModeReadOnly Mode = "read-only"
)

type TargetKind string

const (
	TargetFeature    TargetKind = "feature"
	TargetLegacyMain TargetKind = "legacy-main"
)

type ResultObligations struct {
	Path   string `json:"path"`
	Schema string `json:"schema"`
}

// Contract is target-free authoring input. TargetClaims exists only to reject
// untrusted typed input that attempts to seize live target authority.
type Contract struct {
	Version       string            `json:"version"`
	RouteIntent   string            `json:"route_intent"`
	Mode          Mode              `json:"mode"`
	WritePaths    []string          `json:"write_paths"`
	ReadOnlyPaths []string          `json:"read_only_paths"`
	Goal          string            `json:"goal"`
	DoneCriteria  []string          `json:"done_criteria"`
	HardStops     []string          `json:"hard_stops"`
	Result        ResultObligations `json:"result"`
	TargetClaims  map[string]string `json:"-"`
}
type FeatureTarget struct {
	Feature           string
	ParentRef         string
	BaseSHA           string
	ExpectedParentSHA string
	LiveParentSHA     string
}
type LegacyMainTarget struct {
	ExpectedParentSHA string
	LiveParentSHA     string
}
type TargetBinding struct {
	Feature    *FeatureTarget
	LegacyMain *LegacyMainTarget
}
type BoundTarget struct {
	Kind              TargetKind `json:"kind"`
	Feature           string     `json:"feature,omitempty"`
	ParentRef         string     `json:"parent_ref"`
	BaseSHA           string     `json:"base_sha,omitempty"`
	ExpectedParentSHA string     `json:"expected_parent_sha"`
}
type Artifact struct {
	Version      string
	Body         []byte
	ContractJSON []byte
	ManifestJSON []byte
	Digest       string
	Binding      BoundTarget
}
type ManualPacket struct {
	Body          []byte
	RouteIntent   string
	ReadOnly      bool
	WritePaths    []string
	ReadOnlyPaths []string
	Binding       TargetBinding
}
type BatchItem struct {
	Manual   *ManualPacket
	Contract *Contract
	Binding  TargetBinding
}
type Diagnostic struct {
	PacketIndex int
	Rank        int
	Field       string
	ItemIndex   int
	Code        string
	Message     string
}

func (d Diagnostic) Key() string {
	return fmt.Sprintf("%d:%d:%s:%d:%s", d.PacketIndex, d.Rank, d.Field, d.ItemIndex, d.Code)
}

type Diagnostics []Diagnostic

func (d Diagnostics) Error() string {
	if len(d) == 0 {
		return "packet authoring admission failed"
	}
	return fmt.Sprintf("packet authoring admission failed: %s: %s", d[0].Code, d[0].Message)
}
