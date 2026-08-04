package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrBootstrapFailed is returned when the config directory cannot be prepared.
var ErrBootstrapFailed = errors.New("anubis: config bootstrap failed")

// dataDirPlaceholder is the magic string the shipped config templates carry
// wherever the live configuration directory belongs.
//
// Why isn't this hardcoded to C:\ProgramData?
//
// Excellent question, it mostly boils down to "we can't have nice things".
// Normally the Program Data folder is in C:\ProgramData, but administrators
// often decide to relocate it, and if the templates say it's C:\ProgramData
// on a machine where it is in D: or whatever, the service tries to load
// data from a folder that does not exist.
//
// Needless to say this is sub-optimal, so we have to do ugly hacks to work
// around this. Any time the magic string is present in the upstream templates,
// replace it with the actual location of the Anubis data directory.
//
// In an ideal world this would be %ANUBIS_DATA_DIR% but YAML fights us here
// and I honestly don't care enough to work around it. Whatever. This is fine.
const dataDirPlaceholder = "__ANUBIS_DATA_DIR__"

// bootstrapConfig describes the configuration directory from the templates
// the .msi installer laid down next to the binary in Program Files.
type bootstrapConfig struct {
	// SrcDir holds the read-only templates, such as the installer's etc folder.
	SrcDir string
	// DestDir is the live config directory. It and any missing parent are
	// created.
	DestDir string
	// Files are the base names to copy from SrcDir into DestDir.
	Files []string
	// DataDir is substituted for dataDirPlaceholder in each copied template.
	// Empty means no substitution.
	DataDir string
}

// runBootstrap hydrates configuration from the installer's templates.
//
// The directory keeps whatever permissions it inherits from %ProgramData%,
// which on a stock install means administrators and SYSTEM get full control
// and local users get read access. Anubis does not narrow that, so treat
// anubis.env as readable by anyone with a local account: on a multi-user
// machine, supply the signing key through the environment rather than the
// file.
//
// A file that already exists is never overwritten because it holds whatever
// the administrator configured. It would be a bad user experience to nuke
// configuration on upgrades.
func runBootstrap(cfg bootstrapConfig) error {
	if err := os.MkdirAll(cfg.DestDir, 0o755); err != nil {
		return fmt.Errorf("%w: cannot create %s: %w", ErrBootstrapFailed, cfg.DestDir, err)
	}

	for _, name := range cfg.Files {
		src := filepath.Join(cfg.SrcDir, name)
		dest := filepath.Join(cfg.DestDir, name)

		if err := copyTemplate(src, dest, cfg.DataDir); err != nil {
			return fmt.Errorf("%w: %w", ErrBootstrapFailed, err)
		}
	}

	return nil
}

// copyTemplate copies src to dest, substituting dataDir for every
// dataDirPlaceholder on the way through. A dest that already exists is left
// exactly as it is.
func copyTemplate(src, dest, dataDir string) error {
	// Read the template first. Creating the destination and then failing to
	// find the source would leave an empty file behind, and because this
	// function never overwrites, that empty file would be permanent.
	body, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("cannot read template %s: %w", src, err)
	}

	if dataDir != "" {
		body = bytes.ReplaceAll(body, []byte(dataDirPlaceholder), []byte(dataDir))
	}

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return fmt.Errorf("cannot create %s: %w", dest, err)
	}
	defer out.Close()

	if _, err := out.Write(body); err != nil {
		return fmt.Errorf("cannot write %s: %w", dest, err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("cannot close %s: %w", dest, err)
	}

	return nil
}
