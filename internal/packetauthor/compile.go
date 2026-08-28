package packetauthor

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

type normalizedContract struct {
	Version       string            `json:"version"`
	RouteIntent   string            `json:"route_intent"`
	Mode          Mode              `json:"mode"`
	WritePaths    []string          `json:"write_paths"`
	ReadOnlyPaths []string          `json:"read_only_paths"`
	Goal          string            `json:"goal"`
	DoneCriteria  []string          `json:"done_criteria"`
	HardStops     []string          `json:"hard_stops"`
	Result        ResultObligations `json:"result"`
}
type manifest struct {
	Version      string      `json:"version"`
	ContractHash string      `json:"contract_hash"`
	Binding      BoundTarget `json:"binding"`
}

func Compile(contract Contract, binding TargetBinding) (Artifact, error) {
	normalized, diagnostics := validateContract(contract)
	bound, targetDiagnostics := validateBinding(binding)
	diagnostics = append(diagnostics, targetDiagnostics...)
	if err := diagnosticsError(diagnostics); err != nil {
		return Artifact{}, err
	}
	contractJSON := encodeJSON(normalized)
	contractHash := digest("lucind:packet-author/v1", contractJSON)
	manifestJSON := encodeJSON(manifest{Version: ManifestVersion, ContractHash: contractHash, Binding: bound})
	body := renderBody(normalized)
	return Artifact{
		Version: ContractVersion, Body: body, ContractJSON: contractJSON,
		ManifestJSON: manifestJSON, Digest: digest("lucind:packet-artifact/v1", contractJSON, manifestJSON, body), Binding: bound,
	}, nil
}

func validateContract(contract Contract) (normalizedContract, Diagnostics) {
	var diagnostics Diagnostics
	if contract.Version != ContractVersion {
		diagnostics = append(diagnostics, diagnostic(10, "version", CodeContractInvalid, "contract version must be packet-author/v1"))
	}
	if strings.TrimSpace(contract.Goal) == "" || hasEmptyOrDuplicate(contract.DoneCriteria) || hasEmptyOrDuplicate(contract.HardStops) {
		diagnostics = append(diagnostics, diagnostic(10, "contract", CodeContractInvalid, "goal, criteria, and stops must be non-empty and unique"))
	}
	if contract.Result.Path != ".lucind/result.json" {
		diagnostics = append(diagnostics, diagnostic(20, "result.path", CodeResultPathMissing, "result path must be .lucind/result.json"))
	}
	if contract.Result.Schema != ".lucind/result.schema.json" {
		diagnostics = append(diagnostics, diagnostic(30, "result.schema", CodeResultSchemaMissing, "result schema must be .lucind/result.schema.json"))
	}
	if strings.TrimSpace(contract.RouteIntent) == "" {
		diagnostics = append(diagnostics, diagnostic(40, "route_intent", CodeRouteInvalid, "route intent is required"))
	}
	if contract.Mode != ModeWrite && contract.Mode != ModeReadOnly {
		diagnostics = append(diagnostics, diagnostic(50, "mode", CodeModeCommitConflict, "mode must be write or read-only"))
	}
	if contract.Mode == ModeReadOnly && len(contract.WritePaths) > 0 {
		diagnostics = append(diagnostics, diagnostic(50, "mode", CodeModeCommitConflict, "read-only contracts cannot declare write paths"))
	}
	claimKeys := make([]string, 0, len(contract.TargetClaims))
	for key, value := range contract.TargetClaims {
		if value != "" {
			claimKeys = append(claimKeys, key)
		}
	}
	sort.Strings(claimKeys)
	for _, key := range claimKeys {
		diagnostics = append(diagnostics, diagnostic(60, key, CodeForbiddenTarget, "authored contracts cannot declare live targets"))
	}
	writePaths, pathDiagnostics := normalizePaths("write_paths", contract.WritePaths)
	diagnostics = append(diagnostics, pathDiagnostics...)
	readOnlyPaths, pathDiagnostics := normalizePaths("read_only_paths", contract.ReadOnlyPaths)
	diagnostics = append(diagnostics, pathDiagnostics...)
	return normalizedContract{
		Version: ContractVersion, RouteIntent: contract.RouteIntent, Mode: contract.Mode,
		WritePaths: writePaths, ReadOnlyPaths: readOnlyPaths, Goal: contract.Goal,
		DoneCriteria: append([]string(nil), contract.DoneCriteria...), HardStops: append([]string(nil), contract.HardStops...), Result: contract.Result,
	}, diagnostics
}

func validateBinding(binding TargetBinding) (BoundTarget, Diagnostics) {
	if (binding.Feature == nil) == (binding.LegacyMain == nil) {
		return BoundTarget{}, Diagnostics{diagnostic(70, "target", CodeTargetIncomplete, "exactly one typed target is required")}
	}
	if feature := binding.Feature; feature != nil {
		complete := feature.Feature != "" && validFeatureRef(feature.ParentRef) && validSHA(feature.BaseSHA) && validSHA(feature.ExpectedParentSHA) && validSHA(feature.LiveParentSHA)
		if !complete {
			return BoundTarget{}, Diagnostics{diagnostic(70, "target", CodeTargetIncomplete, "feature target is incomplete or unauthorized")}
		}
		bound := BoundTarget{Kind: TargetFeature, Feature: feature.Feature, ParentRef: feature.ParentRef, BaseSHA: feature.BaseSHA, ExpectedParentSHA: feature.ExpectedParentSHA}
		if feature.ExpectedParentSHA != feature.LiveParentSHA {
			return bound, Diagnostics{diagnostic(80, "expected_parent_sha", CodeTargetStale, "feature parent changed before admission")}
		}
		return bound, nil
	}
	legacy := binding.LegacyMain
	if !validSHA(legacy.ExpectedParentSHA) || !validSHA(legacy.LiveParentSHA) {
		return BoundTarget{}, Diagnostics{diagnostic(70, "target", CodeTargetIncomplete, "legacy-main target is incomplete")}
	}
	bound := BoundTarget{Kind: TargetLegacyMain, ParentRef: "refs/heads/main", ExpectedParentSHA: legacy.ExpectedParentSHA}
	if legacy.ExpectedParentSHA != legacy.LiveParentSHA {
		return bound, Diagnostics{diagnostic(80, "expected_parent_sha", CodeTargetStale, "main changed before admission")}
	}
	return bound, nil
}

func normalizePaths(field string, paths []string) ([]string, Diagnostics) {
	normalized := make([]string, 0, len(paths))
	var diagnostics Diagnostics
	for i, value := range paths {
		if value == "" || strings.ContainsAny(value, "\\\x00") || strings.HasPrefix(value, "/") || path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") {
			d := diagnostic(90, field, CodePathInvalid, "path must be normalized and repository-relative")
			d.ItemIndex = i
			diagnostics = append(diagnostics, d)
			continue
		}
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized, diagnostics
}

func hasEmptyOrDuplicate(items []string) bool {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item == "" {
			return true
		}
		if _, exists := seen[item]; exists {
			return true
		}
		seen[item] = struct{}{}
	}
	return len(items) == 0
}

func validFeatureRef(ref string) bool {
	return strings.HasPrefix(ref, "refs/heads/") && ref != "refs/heads/main" && !strings.HasPrefix(ref, "refs/heads/lucind/")
}

func validSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func encodeJSON(value any) []byte {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		panic(fmt.Sprintf("packetauthor: encode validated value: %v", err))
	}
	return out.Bytes()
}

func renderBody(contract normalizedContract) []byte {
	var out strings.Builder
	fmt.Fprintf(&out, "# Goal\n%s\n\n## Done criteria\n", contract.Goal)
	for _, criterion := range contract.DoneCriteria {
		fmt.Fprintf(&out, "- %s\n", criterion)
	}
	out.WriteString("\n## Hard stops\n")
	for _, stop := range contract.HardStops {
		fmt.Fprintf(&out, "- %s\n", stop)
	}
	fmt.Fprintf(&out, "\n## Return\n```lucind-result-contract\nversion: 1\npath: %s\nschema: %s\nmode: %s\ncommit: %s\n```\n", contract.Result.Path, contract.Result.Schema, contract.Mode, commitForMode(contract.Mode))
	return []byte(out.String())
}

func commitForMode(mode Mode) string {
	if mode == ModeReadOnly {
		return "forbidden"
	}
	return "required"
}

func digest(domain string, fields ...[]byte) string {
	hash := sha256.New()
	writeDigestField(hash, []byte(domain))
	for _, field := range fields {
		writeDigestField(hash, field)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type digestWriter interface{ Write([]byte) (int, error) }

func writeDigestField(writer digestWriter, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = writer.Write(size[:])
	_, _ = writer.Write(value)
}
