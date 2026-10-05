package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/LanzerDevCorp/lucind-ai/internal/claudeplugin"
	"github.com/LanzerDevCorp/lucind-ai/internal/skillselect"
	"github.com/LanzerDevCorp/lucind-ai/internal/userconfig"
)

var (
	isStdinTerminal = func() bool { return isTerminal(os.Stdin) }

	readSecretKey = func(stderr io.Writer) (string, error) {
		const prompt = "TYPESAFE_API_KEY (selects skills with Jev; press Enter to skip): "

		// Catch Ctrl-C before echo is turned off, so an interrupt can never leave the terminal silent.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sig)

		hidden := false
		if _, err := exec.LookPath("stty"); err == nil {
			off := exec.Command("stty", "-echo")
			off.Stdin = os.Stdin
			hidden = off.Run() == nil
		}
		if hidden {
			restore := func() {
				on := exec.Command("stty", "echo")
				on.Stdin = os.Stdin
				_ = on.Run()
			}
			defer restore()
			done := make(chan struct{})
			defer close(done)
			go func() {
				select {
				case <-sig:
					restore()
					_, _ = fmt.Fprintln(stderr)
					os.Exit(130)
				case <-done:
				}
			}()
		} else {
			_, _ = fmt.Fprintln(stderr, "stty unavailable; input will be visible")
		}

		_, _ = fmt.Fprint(stderr, prompt)
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if hidden {
			_, _ = fmt.Fprintln(stderr)
		}
		if err != nil && len(line) == 0 {
			if errors.Is(err, io.EOF) {
				// Ctrl-D on an empty prompt is a skip, not a failure.
				return "", nil
			}
			return "", err
		}
		return strings.TrimSpace(line), nil
	}

	writeUserKey  = userconfig.WriteKey
	resolveAPIKey = skillselect.ResolveKey
)

// isTerminal reports whether f is an interactive terminal. A character device is not enough:
// /dev/null is one and is what an agent or a non-interactive shell gives as stdin, so it is
// excluded explicitly, and stty (which fails with ENOTTY on anything that is not a terminal)
// has the final word when it is available.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if null, err := os.Stat(os.DevNull); err == nil && os.SameFile(fi, null) {
		return false
	}
	if _, err := exec.LookPath("stty"); err != nil {
		return true
	}
	probe := exec.Command("stty", "-g")
	probe.Stdin = f
	return probe.Run() == nil
}

// resetStoredKey replaces the key stored in the user config after asking for a new one, which plain
// install never does because it trusts any key that resolves. An empty answer keeps the current setup.
func resetStoredKey(stdout, stderr io.Writer) (claudeplugin.Variant, error) {
	if !isStdinTerminal() {
		return claudeplugin.VariantManual, errors.New("--reset-key needs a terminal to ask for the new key")
	}
	_, _ = fmt.Fprintln(stderr, "lucind-ai: replacing the stored TYPESAFE_API_KEY; press Enter to keep the current one")
	answer, err := readSecretKey(stderr)
	if err != nil {
		return claudeplugin.VariantManual, fmt.Errorf("read API key: %w", err)
	}
	if answer == "" {
		if resolveAPIKey() != "" {
			return claudeplugin.VariantAuto, nil
		}
		return claudeplugin.VariantManual, nil
	}
	if err := writeUserKey("TYPESAFE_API_KEY", answer); err != nil {
		return claudeplugin.VariantManual, fmt.Errorf("store API key: %w", err)
	}
	if path, err := userconfig.EnvFile(); err == nil {
		_, _ = fmt.Fprintf(stdout, "lucind-ai: stored the new TYPESAFE_API_KEY in %s\n", path)
	}
	if os.Getenv("TYPESAFE_API_KEY") != "" {
		_, _ = fmt.Fprintln(stderr, "lucind-ai: the TYPESAFE_API_KEY environment variable is set and overrides the stored key; update or unset it too")
	}
	return claudeplugin.VariantAuto, nil
}

func determineVariantAndSetupKey(stdout, stderr io.Writer, resetKey bool) (claudeplugin.Variant, error) {
	if resetKey {
		return resetStoredKey(stdout, stderr)
	}
	if key := resolveAPIKey(); key != "" {
		return claudeplugin.VariantAuto, nil
	}

	if isStdinTerminal() {
		answer, err := readSecretKey(stderr)
		if err != nil {
			return claudeplugin.VariantManual, fmt.Errorf("read API key: %w", err)
		}
		if answer != "" {
			if err := writeUserKey("TYPESAFE_API_KEY", answer); err != nil {
				return claudeplugin.VariantManual, fmt.Errorf("store API key: %w", err)
			}
			return claudeplugin.VariantAuto, nil
		}
		return claudeplugin.VariantManual, nil
	}

	_, _ = fmt.Fprintln(stdout, "lucind-ai: TYPESAFE_API_KEY is not configured; run lucind-ai install in a terminal or put TYPESAFE_API_KEY=... in ~/.config/lucind/env (re-running install switches the skill)")
	return claudeplugin.VariantManual, nil
}
