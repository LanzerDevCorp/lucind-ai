package ledger

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/LanzerDevCorp/lucind-ai/internal/candidatechange"
)

const (
	AuthoringEvidenceVersion = "lane-authoring-evidence/v1"
	LegacyAuthoringVersion   = "legacy/v1"
)

// AuthoringEvidence is the versioned frozen candidate contract. Hash is
// computed over this exact JSON shape with Hash empty.
type AuthoringEvidence struct {
	Version          string                   `json:"version"`
	Hash             string                   `json:"hash"`
	PacketDigest     string                   `json:"packet_digest"`
	AuthoringMode    string                   `json:"authoring_mode"`
	ContractVersion  string                   `json:"contract_version"`
	Contract         json.RawMessage          `json:"contract"`
	Binding          json.RawMessage          `json:"binding"`
	Mode             string                   `json:"mode"`
	CommitObligation string                   `json:"commit_obligation"`
	WritePaths       []string                 `json:"write_paths"`
	ReadOnlyPaths    []string                 `json:"read_only_paths"`
	DoneCriteria     []string                 `json:"done_criteria"`
	HardStops        []string                 `json:"hard_stops"`
	ResultPath       string                   `json:"result_path"`
	ResultSchema     string                   `json:"result_schema"`
	BaseCommit       string                   `json:"base_commit"`
	BaseTree         string                   `json:"base_tree"`
	CandidateCommit  string                   `json:"candidate_commit"`
	CandidateTree    string                   `json:"candidate_tree"`
	Changes          []candidatechange.Change `json:"changes"`
	ResultHash       string                   `json:"result_hash"`
}

func FreezeAuthoringEvidence(e AuthoringEvidence) (string, string, error) {
	e.Version, e.Hash = AuthoringEvidenceVersion, ""
	payload, err := json.Marshal(e)
	if err != nil {
		return "", "", err
	}
	h := sha256.New()
	var size [8]byte
	for _, value := range [][]byte{[]byte("lucind:lane-authoring-evidence/v1"), payload} {
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = h.Write(size[:])
		_, _ = h.Write(value)
	}
	e.Hash = "sha256:" + hex.EncodeToString(h.Sum(nil))
	encoded, err := json.Marshal(e)
	return string(encoded), e.Hash, err
}

func DecodeAuthoringEvidence(version, encoded, hash string) (AuthoringEvidence, error) {
	if version != AuthoringEvidenceVersion || encoded == "" || hash == "" {
		return AuthoringEvidence{}, errors.New("ledger: incomplete authoring evidence")
	}
	var got AuthoringEvidence
	if err := json.Unmarshal([]byte(encoded), &got); err != nil {
		return AuthoringEvidence{}, err
	}
	wantEncoded, wantHash, err := FreezeAuthoringEvidence(got)
	if err != nil || got.Version != version || got.Hash != hash || wantHash != hash || wantEncoded != encoded {
		return AuthoringEvidence{}, errors.New("ledger: authoring evidence hash mismatch")
	}
	return got, nil
}
