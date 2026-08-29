package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/LanzerDevCorp/lucind-ai/internal/lucindconfig"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/packetauthor"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillroots"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillset"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// dispatchAuthoringInput is the admission seam shared by canonical manual
// packets and future typed authoring adapters. Contract is nil for a packet
// loaded from Markdown; only a trusted in-process contract reaches Compile.
type dispatchAuthoringInput struct {
	Packet   packet.Packet
	Contract *packetauthor.Contract
}

var resolveAdmissionRefSHA = func(ctx context.Context, primaryRoot, ref string) (string, error) {
	revision := ref
	if ref != "HEAD" {
		revision = worktree.CanonicalizeRef(ref)
	}
	return worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, primaryRoot, revision)
}

// admitDispatchBatch validates every item and returns no packets unless the
// entire batch is safe. It performs only read-only target resolution.
func admitDispatchBatch(ctx context.Context, primaryRoot string, inputs []dispatchAuthoringInput) ([]packet.Packet, error) {
	cfg, err := lucindconfig.Load(primaryRoot)
	if err != nil {
		return nil, fmt.Errorf("load repository config: %w", err)
	}

	budget := skillset.DefaultSkillBudget
	if cfg.SkillBudget != nil {
		budget = *cfg.SkillBudget
	}

	var resolver *skillroots.Resolver
	rootsCfg, err := skillroots.LoadConfig(filepath.Join(primaryRoot, skillroots.DefaultConfigRelPath))
	if err != nil {
		if errors.Is(err, skillroots.ErrMissingConfig) {
			resolver = skillroots.NewResolver(nil)
		} else {
			return nil, fmt.Errorf("load skill roots config: %w", err)
		}
	} else {
		resolver = skillroots.NewResolver(rootsCfg.Roots)
	}

	packets := make([]packet.Packet, len(inputs))
	manuals := make([]packetauthor.ManualPacket, len(inputs))
	items := make([]packetauthor.BatchItem, len(inputs))
	derivedSkillsPerItem := make([][]string, len(inputs))

	for i, input := range inputs {
		packets[i] = input.Packet
		binding := dispatchTargetBinding(ctx, primaryRoot, input.Packet)

		sddPhase := input.Packet.SDDPhase
		laneRole := input.Packet.LaneRole
		adhocSkills := input.Packet.AdhocSkills

		if input.Contract != nil {
			if sddPhase == "" && skillset.IsValidSDDPhase(input.Contract.RouteIntent) {
				sddPhase = input.Contract.RouteIntent
			}
			if laneRole == "" {
				laneRole = input.Contract.LaneRole
			}
			if len(adhocSkills) == 0 {
				adhocSkills = input.Contract.AdhocSkills
			}
		} else {
			if sddPhase == "" && skillset.IsValidSDDPhase(input.Packet.RoutedBy) {
				sddPhase = input.Packet.RoutedBy
			}
		}

		stackSkills := cfg.StackSkills(laneRole)
		var derived []string
		var resolvedPaths []string
		if sddPhase != "" || laneRole != "" || len(adhocSkills) > 0 || len(stackSkills) > 0 {
			var err error
			derived, err = skillset.Derive(sddPhase, laneRole, stackSkills, adhocSkills)
			if err != nil {
				return nil, fmt.Errorf("derive skills for packet[%d]: %w", i, err)
			}

			if len(derived) > budget {
				return nil, fmt.Errorf("lucind-ai: packet[%d] (%s) required skills count %d exceeds budget %d (skills: %s)",
					i, input.Packet.ID, len(derived), budget, strings.Join(derived, ", "))
			}

			resolvedPaths, err = resolver.ResolvePaths(derived)
			if err != nil {
				return nil, err
			}

			derivedSkillsPerItem[i] = derived
		}

		if input.Contract != nil {
			contractCopy := *input.Contract
			if len(resolvedPaths) > 0 {
				contractCopy.RequiredSkills = resolvedPaths
			}
			items[i] = packetauthor.BatchItem{Contract: &contractCopy, Binding: binding}
			continue
		}
		manuals[i] = packetauthor.ManualPacket{
			Body: []byte(input.Packet.Body), RouteIntent: input.Packet.RoutedBy,
			ReadOnly: input.Packet.ReadOnly, WritePaths: input.Packet.AllowedPaths,
			ReadOnlyPaths: input.Packet.ReadOnlyPaths, Binding: binding,
		}
		items[i] = packetauthor.BatchItem{Manual: &manuals[i]}
	}

	artifacts, err := packetauthor.AdmitBatch(items)
	if err != nil {
		return nil, err
	}

	for i, input := range inputs {
		if input.Contract == nil {
			// AdmitManual returns a defensive copy of the original bytes. Assigning
			// it back is safe and makes the byte-preservation invariant explicit.
			packets[i].Body = string(artifacts[i].Body)
			packets[i].RequiredSkills = append([]string(nil), derivedSkillsPerItem[i]...)
			continue
		}
		var normalized struct {
			RouteIntent    string            `json:"route_intent"`
			Mode           packetauthor.Mode `json:"mode"`
			RequiredSkills []string          `json:"required_skills"`
			WritePaths     []string          `json:"write_paths"`
			ReadOnlyPaths  []string          `json:"read_only_paths"`
		}
		if err := json.Unmarshal(artifacts[i].ContractJSON, &normalized); err != nil {
			return nil, fmt.Errorf("decode admitted packet[%d] contract: %w", i, err)
		}
		bindingJSON, err := json.Marshal(artifacts[i].Binding)
		if err != nil {
			return nil, fmt.Errorf("encode admitted packet[%d] binding: %w", i, err)
		}
		packets[i].Body = string(artifacts[i].Body)
		packets[i].RoutedBy = normalized.RouteIntent
		packets[i].ReadOnly = normalized.Mode == packetauthor.ModeReadOnly
		packets[i].RequiredSkills = append([]string(nil), normalized.RequiredSkills...)
		packets[i].AllowedPaths = append([]string(nil), normalized.WritePaths...)
		packets[i].ReadOnlyPaths = append([]string(nil), normalized.ReadOnlyPaths...)
		packets[i].Authoring = &packet.Authoring{
			ContractVersion: artifacts[i].Version, Digest: artifacts[i].Digest,
			ContractJSON: append([]byte(nil), artifacts[i].ContractJSON...),
			BindingJSON:  bindingJSON,
		}
	}
	return packets, nil
}

func dispatchTargetBinding(ctx context.Context, primaryRoot string, p packet.Packet) packetauthor.TargetBinding {
	if p.LegacyMain {
		live := ""
		if p.ExpectedParentSHA != "" {
			resolved, err := resolveAdmissionRefSHA(ctx, primaryRoot, "HEAD")
			if err == nil {
				live = resolved
			}
		}
		return packetauthor.TargetBinding{LegacyMain: &packetauthor.LegacyMainTarget{
			ExpectedParentSHA: p.ExpectedParentSHA, LiveParentSHA: live,
		}}
	}
	live := ""
	if p.ParentRef != "" && p.ExpectedParentSHA != "" {
		resolved, err := resolveAdmissionRefSHA(ctx, primaryRoot, p.ParentRef)
		if err == nil {
			live = resolved
		}
	}
	return packetauthor.TargetBinding{Feature: &packetauthor.FeatureTarget{
		Feature: p.Feature, ParentRef: p.ParentRef, BaseSHA: p.BaseSHA,
		ExpectedParentSHA: p.ExpectedParentSHA, LiveParentSHA: live,
	}}
}

func printAdmissionError(w io.Writer, err error) {
	if diagnostics, ok := err.(packetauthor.Diagnostics); ok {
		for _, diagnostic := range diagnostics {
			item := ""
			if diagnostic.ItemIndex >= 0 {
				item = fmt.Sprintf("[%d]", diagnostic.ItemIndex)
			}
			message := diagnostic.Message
			if diagnostic.Code == packetauthor.CodeTargetIncomplete {
				message += "; declare a complete feature target or use --legacy-main with --expected-parent-sha"
			}
			fmt.Fprintf(w, "lucind-ai: packet[%d] %s %s%s: %s\n", diagnostic.PacketIndex, diagnostic.Code, diagnostic.Field, item, message)
		}
		return
	}
	fmt.Fprintf(w, "lucind-ai: %v\n", err)
}

// compileSpecialistPacket keeps execution and identity checks in the bounded
// adapter, then gives the target-free result to the trusted compiler.
func compileSpecialistPacket(ctx context.Context, runner packetauthor.SpecialistRunner, source packetauthor.Contract, binding packetauthor.TargetBinding) (packetauthor.Artifact, error) {
	request, err := packetauthor.NewSpecialistRequest(source)
	if err != nil {
		return packetauthor.Artifact{}, err
	}
	contract, err := (packetauthor.SpecialistAdapter{Runner: runner}).Author(ctx, request)
	if err != nil {
		return packetauthor.Artifact{}, err
	}
	return packetauthor.Compile(contract, binding)
}
