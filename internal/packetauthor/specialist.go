package packetauthor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	SpecialistAgentName = "lucind-packet-author"
	SpecialistVersion   = "packet-author-specialist/v1"
	SpecialistOutputV1  = "packet-author-output/v1"

	CodeSpecialistAuthority = "PA_SPECIALIST_AUTHORITY_FORBIDDEN"
	CodeSpecialistRender    = "PA_SPECIALIST_RENDER_FORBIDDEN"
	CodeSpecialistOutput    = "PA_SPECIALIST_OUTPUT_INVALID"
	CodeSpecialistDuplicate = "PA_SPECIALIST_OUTPUT_DUPLICATE"
	CodeSpecialistIdentity  = "PA_SPECIALIST_IDENTITY_INVALID"
	CodeSpecialistRequest   = "PA_SPECIALIST_REQUEST_INVALID"
)

type SpecialistRequest struct {
	Version  string   `json:"version"`
	Contract Contract `json:"contract"`
}

type SpecialistInvocation struct {
	Agent string
	Input []byte
}

type SpecialistResponse struct {
	Identity string
	Output   []byte
}

type SpecialistRunner interface {
	Run(context.Context, SpecialistInvocation) (SpecialistResponse, error)
}

type SpecialistAdapter struct {
	Runner SpecialistRunner
}

type SpecialistError struct {
	Code  string
	Field string
	Cause string
}

func (e *SpecialistError) Error() string {
	if e.Cause == "" {
		return e.Code + ": " + e.Field
	}
	return e.Code + ": " + e.Field + ": " + e.Cause
}

func NewSpecialistRequest(contract Contract) (SpecialistRequest, error) {
	request := SpecialistRequest{Version: SpecialistVersion, Contract: contract}
	if err := validateSpecialistRequest(request); err != nil {
		return SpecialistRequest{}, err
	}
	request.Contract = cloneContract(contract)
	return request, nil
}

func DecodeSpecialistOutput(data []byte) (Contract, error) {
	fields, err := inspectSpecialistJSON(data)
	if err != nil {
		return Contract{}, err
	}
	for _, field := range fields {
		name := normalizeSpecialistField(field)
		if forbiddenSpecialistAuthority[name] {
			return Contract{}, specialistError(CodeSpecialistAuthority, field, "specialist output cannot choose runtime authority")
		}
		if forbiddenSpecialistRender[name] {
			return Contract{}, specialistError(CodeSpecialistRender, field, "only the trusted compiler renders packets")
		}
	}
	var output struct {
		Version  string   `json:"version"`
		Contract Contract `json:"contract"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&output); err != nil {
		return Contract{}, specialistError(CodeSpecialistOutput, "output", err.Error())
	}
	if output.Version != SpecialistOutputV1 {
		return Contract{}, specialistError(CodeSpecialistOutput, "version", "must be "+SpecialistOutputV1)
	}
	if diagnostics := contractDiagnostics(output.Contract); diagnostics != nil {
		return Contract{}, specialistError(CodeSpecialistOutput, "contract", diagnostics.Error())
	}
	return cloneContract(output.Contract), nil
}

func (a SpecialistAdapter) Author(ctx context.Context, request SpecialistRequest) (Contract, error) {
	if err := validateSpecialistRequest(request); err != nil {
		return Contract{}, err
	}
	if a.Runner == nil {
		return Contract{}, specialistError(CodeSpecialistRequest, "runner", "runner is required")
	}
	input := encodeJSON(request)
	response, err := a.Runner.Run(ctx, SpecialistInvocation{Agent: SpecialistAgentName, Input: input})
	if err != nil {
		return Contract{}, err
	}
	if response.Identity != SpecialistAgentName {
		return Contract{}, specialistError(CodeSpecialistIdentity, "identity", fmt.Sprintf("got %q, want %q", response.Identity, SpecialistAgentName))
	}
	return DecodeSpecialistOutput(response.Output)
}

func validateSpecialistRequest(request SpecialistRequest) error {
	if request.Version != SpecialistVersion {
		return specialistError(CodeSpecialistRequest, "version", "must be "+SpecialistVersion)
	}
	if len(request.Contract.TargetClaims) != 0 {
		return specialistError(CodeSpecialistAuthority, "contract.target_claims", "specialist requests are target-free")
	}
	if diagnostics := contractDiagnostics(request.Contract); diagnostics != nil {
		return specialistError(CodeSpecialistRequest, "contract", diagnostics.Error())
	}
	return nil
}

func contractDiagnostics(contract Contract) Diagnostics {
	_, diagnostics := validateContract(contract)
	if len(diagnostics) == 0 {
		return nil
	}
	return orderDiagnostics(diagnostics)
}

func cloneContract(contract Contract) Contract {
	contract.WritePaths = append([]string(nil), contract.WritePaths...)
	contract.ReadOnlyPaths = append([]string(nil), contract.ReadOnlyPaths...)
	contract.DoneCriteria = append([]string(nil), contract.DoneCriteria...)
	contract.HardStops = append([]string(nil), contract.HardStops...)
	contract.TargetClaims = nil
	return contract
}

func specialistError(code, field, cause string) error {
	return &SpecialistError{Code: code, Field: field, Cause: cause}
}

var forbiddenSpecialistAuthority = map[string]bool{
	"target": true, "targetsha": true, "targetclaims": true, "binding": true,
	"feature": true, "parentref": true, "basesha": true, "expectedparentsha": true, "liveparentsha": true,
	"commit": true, "commitsha": true, "dispatch": true, "executor": true, "agent": true, "model": true,
	"worktree": true, "worktreepath": true, "quota": true, "allocation": true, "allocate": true,
	"integration": true, "integrate": true, "acceptance": true, "promotion": true, "promote": true,
}

var forbiddenSpecialistRender = map[string]bool{
	"markdown": true, "frontmatter": true, "body": true, "packet": true, "rendered": true, "renderedpacket": true,
}

func normalizeSpecialistField(field string) string {
	if index := strings.LastIndexByte(field, '.'); index >= 0 {
		field = field[index+1:]
	}
	return strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(field))
}

func inspectSpecialistJSON(data []byte) ([]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	fields, err := inspectSpecialistValue(decoder, "")
	if err != nil {
		var specialistErr *SpecialistError
		if errors.As(err, &specialistErr) {
			return nil, err
		}
		return nil, specialistError(CodeSpecialistOutput, "output", err.Error())
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return nil, specialistError(CodeSpecialistDuplicate, "output", "multiple JSON values")
		}
		return nil, specialistError(CodeSpecialistOutput, "output", err.Error())
	}
	return fields, nil
}

func inspectSpecialistValue(decoder *json.Decoder, path string) ([]string, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil, nil
	}
	var fields []string
	switch delim {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("object key is not a string")
			}
			field := key
			if path != "" {
				field = path + "." + key
			}
			duplicateKey := strings.ToLower(key)
			if seen[duplicateKey] {
				return nil, specialistError(CodeSpecialistDuplicate, field, "duplicate JSON field")
			}
			seen[duplicateKey] = true
			fields = append(fields, field)
			nested, err := inspectSpecialistValue(decoder, field)
			if err != nil {
				return nil, err
			}
			fields = append(fields, nested...)
		}
	case '[':
		for decoder.More() {
			nested, err := inspectSpecialistValue(decoder, path)
			if err != nil {
				return nil, err
			}
			fields = append(fields, nested...)
		}
	default:
		return nil, fmt.Errorf("unexpected delimiter %q", delim)
	}
	_, err = decoder.Token()
	return fields, err
}
