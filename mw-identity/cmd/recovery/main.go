package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/config"
	"github.com/vtmattedi/mw-plane-control/mw-identity/internal/recovery"
	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: recovery create|validate|prepare [options]")
	}
	action := os.Args[1]
	if action != "create" && action != "validate" && action != "prepare" {
		return fmt.Errorf("unknown recovery action")
	}
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	archive := flags.String("archive", "", "encrypted archive path (new output for create)")
	output := flags.String("output-dir", "", "new isolated output directory for prepare")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	if *archive == "" || flags.NArg() != 0 {
		return fmt.Errorf("--archive is required; unexpected positional arguments are refused")
	}
	if action == "prepare" && *output == "" {
		return fmt.Errorf("--output-dir is required")
	}
	if action != "prepare" && *output != "" {
		return fmt.Errorf("--output-dir only applies to prepare")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("invalid deployment configuration")
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("a trusted interactive terminal is required for the recovery passphrase")
	}
	fmt.Fprint(os.Stderr, "Recovery passphrase: ")
	secret, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	defer clear(secret)
	if action == "create" {
		fmt.Fprint(os.Stderr, "Confirm passphrase: ")
		second, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return err
		}
		matches := string(secret) == string(second)
		clear(second)
		if !matches {
			return fmt.Errorf("passphrases differ")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var m recovery.Manifest
	switch action {
	case "create":
		m, err = recovery.Create(ctx, cfg, *archive, string(secret))
	case "validate":
		m, err = recovery.Validate(ctx, cfg, *archive, string(secret))
	case "prepare":
		m, err = recovery.Prepare(ctx, cfg, *archive, string(secret), *output)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(m)
}
