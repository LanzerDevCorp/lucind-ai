package claudemd_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/claudemd"
)

const sampleBlock = `<!-- lucind:dispatch -->
## lucind-ai Dispatch (user-owned, takes precedence)
Sample block content.
<!-- /lucind:dispatch -->
`

func TestInstall_FileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "subdir", "CLAUDE.md")

	outcome, err := claudemd.Install(nestedPath, []byte(sampleBlock))
	if err != nil {
		t.Fatalf("Install on missing file returned unexpected error: %v", err)
	}

	if outcome != claudemd.OutcomeCreated {
		t.Fatalf("outcome = %q, want %q", outcome, claudemd.OutcomeCreated)
	}

	// File should exist with 0644 mode
	info, err := os.Stat(nestedPath)
	if err != nil {
		t.Fatalf("target file was not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("file mode = %04o, want 0644", perm)
	}

	// Content should match sampleBlock and end with newline
	data, err := os.ReadFile(nestedPath)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if !bytes.Equal(data, []byte(sampleBlock)) {
		t.Fatalf("file content = %q, want %q", string(data), sampleBlock)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Errorf("created file missing trailing newline")
	}

	// Parent directory should exist with 0755 permissions
	parentInfo, err := os.Stat(filepath.Dir(nestedPath))
	if err != nil {
		t.Fatalf("parent directory not created: %v", err)
	}
	if perm := parentInfo.Mode().Perm(); perm != 0o755 {
		t.Errorf("parent dir mode = %04o, want 0755", perm)
	}

	// No backup file should be created for a missing file
	bakPath := nestedPath + ".lucind-ai.bak"
	if _, err := os.Stat(bakPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("backup file should not exist for created file, found err: %v", err)
	}
}

func TestInstall_FileMissing_BlockWithoutTrailingNewline(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "CLAUDE.md")

	blockWithoutNewline := []byte("<!-- lucind:dispatch -->\ncontent\n<!-- /lucind:dispatch -->")

	outcome, err := claudemd.Install(targetPath, blockWithoutNewline)
	if err != nil {
		t.Fatalf("Install returned unexpected error: %v", err)
	}
	if outcome != claudemd.OutcomeCreated {
		t.Fatalf("outcome = %q, want %q", outcome, claudemd.OutcomeCreated)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Errorf("expected trailing newline when block lacked one")
	}
	wantContent := append(blockWithoutNewline, '\n')
	if !bytes.Equal(data, wantContent) {
		t.Fatalf("content = %q, want %q", string(data), string(wantContent))
	}
}

func TestInstall_Idempotency(t *testing.T) {
	t.Run("Missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "CLAUDE.md")

		out1, err := claudemd.Install(target, []byte(sampleBlock))
		if err != nil {
			t.Fatalf("run 1 failed: %v", err)
		}
		if out1 != claudemd.OutcomeCreated {
			t.Fatalf("run 1 outcome = %q, want %q", out1, claudemd.OutcomeCreated)
		}

		bytes1, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read after run 1: %v", err)
		}

		out2, err := claudemd.Install(target, []byte(sampleBlock))
		if err != nil {
			t.Fatalf("run 2 failed: %v", err)
		}
		if out2 != claudemd.OutcomeUnchanged {
			t.Fatalf("run 2 outcome = %q, want %q", out2, claudemd.OutcomeUnchanged)
		}

		bytes2, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read after run 2: %v", err)
		}

		if !bytes.Equal(bytes1, bytes2) {
			t.Fatalf("file bytes changed on second run: %q != %q", string(bytes1), string(bytes2))
		}

		bakPath := target + ".lucind-ai.bak"
		if _, err := os.Stat(bakPath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("backup file should not exist after run 2 on created file")
		}
	})

	t.Run("Append", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "CLAUDE.md")
		initial := []byte("# Header\n@RTK.md\n")
		if err := os.WriteFile(target, initial, 0o644); err != nil {
			t.Fatalf("write initial: %v", err)
		}

		out1, err := claudemd.Install(target, []byte(sampleBlock))
		if err != nil {
			t.Fatalf("run 1 failed: %v", err)
		}
		if out1 != claudemd.OutcomeAppended {
			t.Fatalf("run 1 outcome = %q, want %q", out1, claudemd.OutcomeAppended)
		}

		bytes1, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read after run 1: %v", err)
		}

		bakPath := target + ".lucind-ai.bak"
		bak1, err := os.ReadFile(bakPath)
		if err != nil {
			t.Fatalf("read backup after run 1: %v", err)
		}
		if !bytes.Equal(bak1, initial) {
			t.Fatalf("backup = %q, want %q", string(bak1), string(initial))
		}

		bakInfo1, err := os.Stat(bakPath)
		if err != nil {
			t.Fatalf("stat backup after run 1: %v", err)
		}

		out2, err := claudemd.Install(target, []byte(sampleBlock))
		if err != nil {
			t.Fatalf("run 2 failed: %v", err)
		}
		if out2 != claudemd.OutcomeUnchanged {
			t.Fatalf("run 2 outcome = %q, want %q", out2, claudemd.OutcomeUnchanged)
		}

		bytes2, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read after run 2: %v", err)
		}
		if !bytes.Equal(bytes1, bytes2) {
			t.Fatalf("file bytes changed on second run")
		}

		bakInfo2, err := os.Stat(bakPath)
		if err != nil {
			t.Fatalf("stat backup after run 2: %v", err)
		}
		if bakInfo1.ModTime() != bakInfo2.ModTime() {
			t.Errorf("backup file was touched during second unchanged run")
		}
	})

	t.Run("Replace", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "CLAUDE.md")
		initial := []byte("# Header\n<!-- lucind:dispatch -->\nold block\n<!-- /lucind:dispatch -->\n# Footer\n")
		if err := os.WriteFile(target, initial, 0o644); err != nil {
			t.Fatalf("write initial: %v", err)
		}

		out1, err := claudemd.Install(target, []byte(sampleBlock))
		if err != nil {
			t.Fatalf("run 1 failed: %v", err)
		}
		if out1 != claudemd.OutcomeReplaced {
			t.Fatalf("run 1 outcome = %q, want %q", out1, claudemd.OutcomeReplaced)
		}

		bytes1, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read after run 1: %v", err)
		}

		out2, err := claudemd.Install(target, []byte(sampleBlock))
		if err != nil {
			t.Fatalf("run 2 failed: %v", err)
		}
		if out2 != claudemd.OutcomeUnchanged {
			t.Fatalf("run 2 outcome = %q, want %q", out2, claudemd.OutcomeUnchanged)
		}

		bytes2, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read after run 2: %v", err)
		}
		if !bytes.Equal(bytes1, bytes2) {
			t.Fatalf("file bytes changed on second run")
		}
	})
}

func TestInstall_ContentOutsideMarkersPreservedByteForByte(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "CLAUDE.md")

	prefix := `# Global Configuration
<!-- gentle-ai:user-rules -->
User rule line 1
User rule line 2
<!-- /gentle-ai:user-rules -->

Instruction section
@RTK.md

`
	suffix := `
## Trailing Section
Some final instructions that must not be altered.
`
	initialContent := prefix + "<!-- lucind:dispatch -->\nold dispatch block\n<!-- /lucind:dispatch -->\n" + suffix
	if err := os.WriteFile(target, []byte(initialContent), 0o644); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	newBlock := `<!-- lucind:dispatch -->
## lucind-ai Dispatch (new version)
New rules for dispatching.
<!-- /lucind:dispatch -->
`

	outcome, err := claudemd.Install(target, []byte(newBlock))
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if outcome != claudemd.OutcomeReplaced {
		t.Fatalf("outcome = %q, want %q", outcome, claudemd.OutcomeReplaced)
	}

	result, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}

	wantContent := prefix + newBlock + suffix
	if !bytes.Equal(result, []byte(wantContent)) {
		t.Fatalf("result content mismatch:\nGOT:\n%s\nWANT:\n%s", string(result), wantContent)
	}

	// Verify exact byte equality of prefix and suffix
	if !bytes.HasPrefix(result, []byte(prefix)) {
		t.Errorf("prefix was not preserved byte for byte")
	}
	if !bytes.HasSuffix(result, []byte(suffix)) {
		t.Errorf("suffix was not preserved byte for byte")
	}
}

func TestInstall_AppendWhenNoMarkers(t *testing.T) {
	t.Run("TrailingNewlineOnExisting", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "CLAUDE.md")
		initial := []byte("# Header\n@RTK.md\n")
		if err := os.WriteFile(target, initial, 0o644); err != nil {
			t.Fatalf("write initial: %v", err)
		}

		outcome, err := claudemd.Install(target, []byte(sampleBlock))
		if err != nil {
			t.Fatalf("Install failed: %v", err)
		}
		if outcome != claudemd.OutcomeAppended {
			t.Fatalf("outcome = %q, want %q", outcome, claudemd.OutcomeAppended)
		}

		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read result: %v", err)
		}

		want := "# Header\n@RTK.md\n\n" + sampleBlock
		if !bytes.Equal(data, []byte(want)) {
			t.Fatalf("got:\n%q\nwant:\n%q", string(data), want)
		}
		if !bytes.HasSuffix(data, []byte("\n")) {
			t.Errorf("missing trailing newline")
		}
	})

	t.Run("NoTrailingNewlineOnExisting", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "CLAUDE.md")
		initial := []byte("# Header\n@RTK.md")
		if err := os.WriteFile(target, initial, 0o644); err != nil {
			t.Fatalf("write initial: %v", err)
		}

		outcome, err := claudemd.Install(target, []byte(sampleBlock))
		if err != nil {
			t.Fatalf("Install failed: %v", err)
		}
		if outcome != claudemd.OutcomeAppended {
			t.Fatalf("outcome = %q, want %q", outcome, claudemd.OutcomeAppended)
		}

		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read result: %v", err)
		}

		want := "# Header\n@RTK.md\n\n" + sampleBlock
		if !bytes.Equal(data, []byte(want)) {
			t.Fatalf("got:\n%q\nwant:\n%q", string(data), want)
		}
	})

	t.Run("MultipleTrailingNewlinesOnExisting", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "CLAUDE.md")
		initial := []byte("# Header\n@RTK.md\n\n\n\n")
		if err := os.WriteFile(target, initial, 0o644); err != nil {
			t.Fatalf("write initial: %v", err)
		}

		outcome, err := claudemd.Install(target, []byte(sampleBlock))
		if err != nil {
			t.Fatalf("Install failed: %v", err)
		}
		if outcome != claudemd.OutcomeAppended {
			t.Fatalf("outcome = %q, want %q", outcome, claudemd.OutcomeAppended)
		}

		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read result: %v", err)
		}

		// Exactly one blank line between existing and block
		want := "# Header\n@RTK.md\n\n" + sampleBlock
		if !bytes.Equal(data, []byte(want)) {
			t.Fatalf("got:\n%q\nwant:\n%q", string(data), want)
		}
	})
}

func TestInstall_SymlinkCase(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "resolved_claude.md")
	symlinkFile := filepath.Join(tmpDir, "CLAUDE.md")

	initial := []byte("# Initial Target Content\n@RTK.md\n")
	if err := os.WriteFile(targetFile, initial, 0o644); err != nil {
		t.Fatalf("write initial target: %v", err)
	}

	if err := os.Symlink(targetFile, symlinkFile); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	outcome, err := claudemd.Install(symlinkFile, []byte(sampleBlock))
	if err != nil {
		t.Fatalf("Install on symlink failed: %v", err)
	}
	if outcome != claudemd.OutcomeAppended {
		t.Fatalf("outcome = %q, want %q", outcome, claudemd.OutcomeAppended)
	}

	// Symlink must still exist as a symlink
	linkFi, err := os.Lstat(symlinkFile)
	if err != nil {
		t.Fatalf("lstat symlink: %v", err)
	}
	if linkFi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink file is no longer a symlink; mode = %v", linkFi.Mode())
	}

	// Symlink destination must still be targetFile
	dest, err := os.Readlink(symlinkFile)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if dest != targetFile {
		t.Fatalf("symlink dest = %q, want %q", dest, targetFile)
	}

	// Target file must have updated content
	targetData, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("read target file: %v", err)
	}
	wantContent := "# Initial Target Content\n@RTK.md\n\n" + sampleBlock
	if !bytes.Equal(targetData, []byte(wantContent)) {
		t.Fatalf("target content = %q, want %q", string(targetData), wantContent)
	}

	// Backup must be at targetFile.lucind-ai.bak, NOT symlinkFile.lucind-ai.bak
	targetBak := targetFile + ".lucind-ai.bak"
	bakData, err := os.ReadFile(targetBak)
	if err != nil {
		t.Fatalf("read target backup: %v", err)
	}
	if !bytes.Equal(bakData, initial) {
		t.Fatalf("backup content = %q, want %q", string(bakData), string(initial))
	}

	symlinkBak := symlinkFile + ".lucind-ai.bak"
	if _, err := os.Lstat(symlinkBak); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("backup file should NOT exist at symlink path %s", symlinkBak)
	}
}

func TestInstall_BackupCreationAndOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "CLAUDE.md")

	v1 := []byte("# Version 1 Content\n@RTK.md\n")
	if err := os.WriteFile(target, v1, 0o644); err != nil {
		t.Fatalf("write v1: %v", err)
	}

	block1 := []byte("<!-- lucind:dispatch -->\nBlock Version 1\n<!-- /lucind:dispatch -->\n")
	out1, err := claudemd.Install(target, block1)
	if err != nil {
		t.Fatalf("install 1 failed: %v", err)
	}
	if out1 != claudemd.OutcomeAppended {
		t.Fatalf("outcome 1 = %q, want %q", out1, claudemd.OutcomeAppended)
	}

	bakPath := target + ".lucind-ai.bak"
	bak1, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("read bak 1: %v", err)
	}
	if !bytes.Equal(bak1, v1) {
		t.Fatalf("bak 1 = %q, want %q", string(bak1), string(v1))
	}

	block2 := []byte("<!-- lucind:dispatch -->\nBlock Version 2\n<!-- /lucind:dispatch -->\n")
	contentBefore2, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read content before 2: %v", err)
	}

	out2, err := claudemd.Install(target, block2)
	if err != nil {
		t.Fatalf("install 2 failed: %v", err)
	}
	if out2 != claudemd.OutcomeReplaced {
		t.Fatalf("outcome 2 = %q, want %q", out2, claudemd.OutcomeReplaced)
	}

	bak2, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("read bak 2: %v", err)
	}
	if !bytes.Equal(bak2, contentBefore2) {
		t.Fatalf("bak 2 = %q, want %q", string(bak2), string(contentBefore2))
	}
}

func TestInstall_MalformedMarkers(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "only begin",
			content: "# Header\n<!-- lucind:dispatch -->\nMissing end marker\n",
		},
		{
			name:    "only end",
			content: "# Header\nMissing begin marker\n<!-- /lucind:dispatch -->\n",
		},
		{
			name:    "end before begin",
			content: "# Header\n<!-- /lucind:dispatch -->\nMiddle\n<!-- lucind:dispatch -->\n",
		},
		{
			name:    "duplicate begin",
			content: "# Header\n<!-- lucind:dispatch -->\nOne\n<!-- lucind:dispatch -->\nTwo\n<!-- /lucind:dispatch -->\n",
		},
		{
			name:    "duplicate end",
			content: "# Header\n<!-- lucind:dispatch -->\nOne\n<!-- /lucind:dispatch -->\nTwo\n<!-- /lucind:dispatch -->\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			target := filepath.Join(tmpDir, "CLAUDE.md")
			origBytes := []byte(tt.content)
			if err := os.WriteFile(target, origBytes, 0o644); err != nil {
				t.Fatalf("write initial: %v", err)
			}

			outcome, err := claudemd.Install(target, []byte(sampleBlock))
			if err == nil {
				t.Fatalf("expected error for %s, got outcome = %q", tt.name, outcome)
			}
			if !errors.Is(err, claudemd.ErrMalformedMarkers) {
				t.Errorf("error = %v, want errors.Is(err, claudemd.ErrMalformedMarkers)", err)
			}

			// Must write nothing: file content unchanged
			afterBytes, err := os.ReadFile(target)
			if err != nil {
				t.Fatalf("read target after error: %v", err)
			}
			if !bytes.Equal(origBytes, afterBytes) {
				t.Errorf("file content was modified despite error")
			}

			// No backup file created
			bakPath := target + ".lucind-ai.bak"
			if _, err := os.Stat(bakPath); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("backup file should not exist on error")
			}

			// No temp files left in directory
			entries, err := os.ReadDir(tmpDir)
			if err != nil {
				t.Fatalf("read dir: %v", err)
			}
			for _, entry := range entries {
				if entry.Name() != "CLAUDE.md" {
					t.Errorf("unexpected file in directory after error: %s", entry.Name())
				}
			}
		})
	}
}

func TestInstall_OutcomeUnchanged_AlreadyIdentical(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "CLAUDE.md")

	initial := []byte("# Header\n\n" + sampleBlock)
	if err := os.WriteFile(target, initial, 0o644); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	infoBefore, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	outcome, err := claudemd.Install(target, []byte(sampleBlock))
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if outcome != claudemd.OutcomeUnchanged {
		t.Fatalf("outcome = %q, want %q", outcome, claudemd.OutcomeUnchanged)
	}

	infoAfter, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if infoBefore.ModTime() != infoAfter.ModTime() {
		t.Errorf("file was touched when outcome was unchanged")
	}

	// No backup file should exist
	bakPath := target + ".lucind-ai.bak"
	if _, err := os.Stat(bakPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("backup file should not exist when outcome is unchanged")
	}
}

func TestInstall_PreservesFileMode(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "CLAUDE.md")
	initial := []byte("# Secret CLAUDE.md\n@RTK.md\n")

	// Create file with 0600 mode
	if err := os.WriteFile(target, initial, 0o600); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	outcome, err := claudemd.Install(target, []byte(sampleBlock))
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if outcome != claudemd.OutcomeAppended {
		t.Fatalf("outcome = %q, want %q", outcome, claudemd.OutcomeAppended)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("perm = %04o, want 0600", perm)
	}
}

func TestInstall_EmptyExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "CLAUDE.md")

	// Empty file exists
	if err := os.WriteFile(target, []byte{}, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	outcome, err := claudemd.Install(target, []byte(sampleBlock))
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if outcome != claudemd.OutcomeAppended {
		t.Fatalf("outcome = %q, want %q", outcome, claudemd.OutcomeAppended)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Equal(data, []byte(sampleBlock)) {
		t.Fatalf("got %q, want %q", string(data), sampleBlock)
	}

	// Backup should exist with 0 bytes
	bakPath := target + ".lucind-ai.bak"
	bakData, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if len(bakData) != 0 {
		t.Fatalf("backup len = %d, want 0", len(bakData))
	}
}

func TestInstall_EmptyBlock(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "CLAUDE.md")

	outcome, err := claudemd.Install(target, []byte{})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if outcome != claudemd.OutcomeCreated {
		t.Fatalf("outcome = %q, want %q", outcome, claudemd.OutcomeCreated)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Equal(data, []byte("\n")) {
		t.Fatalf("got %q, want %q", string(data), "\n")
	}
}

func TestInstall_BrokenSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "nonexistent.md")
	symlinkFile := filepath.Join(tmpDir, "CLAUDE.md")

	if err := os.Symlink(targetFile, symlinkFile); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := claudemd.Install(symlinkFile, []byte(sampleBlock))
	if err == nil {
		t.Fatalf("expected error on broken symlink, got nil")
	}
}

