// Package process implements process group supervision, SIGKILL abrupt termination
// to -pgid, and /proc survivor verification for Linux environments.
package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

var (
	// ErrInvalidPGID is returned when an invalid process group ID (<= 1) is supplied.
	ErrInvalidPGID = errors.New("process: invalid pgid (must be > 1)")
	// ErrSurvivingProcesses is returned when descendant processes remain in the process group after termination.
	ErrSurvivingProcesses = errors.New("process: surviving processes found in process group")
)

// KillGroup abruptly terminates all processes in the specified process group by
// sending SIGKILL to -pgid. If the process group does not exist (ESRCH), KillGroup
// returns nil.
func KillGroup(pgid int) error {
	if pgid <= 1 {
		return ErrInvalidPGID
	}

	err := syscall.Kill(-pgid, syscall.SIGKILL)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("process: kill process group -%d: %w", pgid, err)
	}
	return nil
}

// AuditSurvivors scans /proc on Linux for all active, non-zombie processes belonging
// to the target process group (pgrp == pgid). Returns a sorted slice of surviving PIDs.
func AuditSurvivors(pgid int) ([]int, error) {
	if pgid <= 1 {
		return nil, ErrInvalidPGID
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("process: read /proc: %w", err)
	}

	var survivors []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		statPath := filepath.Join("/proc", entry.Name(), "stat")
		data, err := os.ReadFile(statPath)
		if err != nil {
			// Process may have exited between readdir and readfile.
			continue
		}

		line := string(data)
		idx := strings.LastIndex(line, ")")
		if idx == -1 {
			continue
		}

		fields := strings.Fields(line[idx+1:])
		if len(fields) < 3 {
			continue
		}

		state := fields[0]
		pgrpVal, err := strconv.Atoi(fields[2])
		if err != nil || pgrpVal != pgid {
			continue
		}

		// Zombie processes ('Z') have already terminated and are awaiting reaping.
		if state == "Z" {
			continue
		}

		// Probe liveness with signal 0.
		if probeErr := syscall.Kill(pid, 0); probeErr == nil || errors.Is(probeErr, syscall.EPERM) {
			survivors = append(survivors, pid)
		}
	}

	sort.Ints(survivors)
	return survivors, nil
}

// VerifyZeroSurvivors verifies that no live processes remain in the target process group.
func VerifyZeroSurvivors(pgid int) error {
	survivors, err := AuditSurvivors(pgid)
	if err != nil {
		return err
	}
	if len(survivors) > 0 {
		return fmt.Errorf("%w: pgid %d has %d survivor(s): %v", ErrSurvivingProcesses, pgid, len(survivors), survivors)
	}
	return nil
}

// Supervisor provides process group execution and external supervision.
type Supervisor struct{}

// NewSupervisor creates a new process Supervisor.
func NewSupervisor() *Supervisor {
	return &Supervisor{}
}

// Start configures cmd to run in its own process group (Setpgid: true) and starts it.
// Returns the child's process group ID (which is equal to its PID).
func (s *Supervisor) Start(cmd *exec.Cmd) (int, error) {
	if cmd == nil {
		return 0, errors.New("process: cmd is nil")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("process: start command: %w", err)
	}
	if cmd.Process == nil {
		return 0, errors.New("process: command started with nil process")
	}
	return cmd.Process.Pid, nil
}

// Kill abruptly kills the process group with SIGKILL.
func (s *Supervisor) Kill(pgid int) error {
	return KillGroup(pgid)
}

// Audit scans /proc for surviving processes in pgid.
func (s *Supervisor) Audit(pgid int) ([]int, error) {
	return AuditSurvivors(pgid)
}

// VerifyZero verifies zero surviving processes in pgid.
func (s *Supervisor) VerifyZero(pgid int) error {
	return VerifyZeroSurvivors(pgid)
}
