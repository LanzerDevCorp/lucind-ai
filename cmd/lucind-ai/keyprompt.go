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
	isStdinTerminal = func() bool {
		fi, err := os.Stdin.Stat()
		return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
	}

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

func determineVariantAndSetupKey(stdout, stderr io.Writer) (claudeplugin.Variant, error) {
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
