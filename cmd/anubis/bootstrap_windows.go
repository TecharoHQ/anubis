package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrBootstrapFailed is returned when the config directory cannot be prepared.
var ErrBootstrapFailed = errors.New("anubis: config bootstrap failed")

// ErrUntrustedDir is returned when a directory on the way to the config
// directory already exists and is not something this process is willing to
// use: a symbolic link, a junction, a plain file, or a directory owned by
// somebody who is not an administrator.
var ErrUntrustedDir = errors.New("anubis: directory is not trustworthy")

// ErrNotUnderBase is returned when DestDir does not sit underneath BaseDir.
var ErrNotUnderBase = errors.New("anubis: destination is not under the base directory")

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
	// BaseDir is the trusted directory DestDir lives underneath, such as
	// %ProgramData%. It must already exist. It is never created and never
	// checked: the operating system made it, and a machine where it is not
	// trustworthy is already lost. Every directory between BaseDir and
	// DestDir is ours to create, so every one of them is checked.
	BaseDir string
	// DestDir is the live config directory. It and every directory between it
	// and BaseDir are created if they do not exist.
	DestDir string
	// Files are the base names to copy from SrcDir into DestDir.
	Files []string
	// DataDir is substituted for dataDirPlaceholder in each copied template.
	// Empty means no substitution.
	DataDir string
	// Harden restricts access to DestDir once it is populated. It is a field
	// so tests can substitute a recorder for the real platform call.
	Harden func(dir string) error
	// VerifyDir validates a directory that already existed, which is to say
	// one this process did not create and therefore does not own. It must
	// reject anything that is not a real directory owned by a trusted
	// principal. It is a field so tests can substitute a fake, and nil means
	// no platform check, which is correct where the directory is created by a
	// package manager running as root.
	//
	// Callers get the portable checks in checkPlainDir for free either way.
	VerifyDir func(dir string) error
}

// runBootstrap hydrates configuration and restricts access to it.
//
// Normally I'd do this with os.MkdirAll, but in order to paranoidly make sure
// that every folder in this chain both already exists and has system permissions
// we need to manually go up the chain.
//
// The default ACL on %ProgramData% is kinda like the sticky bit in Unix. Any
// time a user creates a folder in there, they become the owner of that folder.
// This means an unprivileged user _could_ create the vendor directory ahead
// of the installer and keep the owner's implicit WRITE_DAC over it.
//
// The destination directory is hardened before anything is written to it
// because the configuration file holds ed25519 signing keys and the default
// permissions in %ProgramData% let every local user read it. I don't really
// know why Anubis would be run on Windows in such a multi-user environment,
// but it's honestly better to be way more paranoid than the situation demands
// because it's easier to become less opinionated than it is to become more
// opinionated.
//
// A file that already exists is never overwritten because it holds whatever
// the administrator configured. It would be a bad user experience to nuke
// configuration on upgrades.
func runBootstrap(cfg bootstrapConfig) error {
	dirs, err := dirChain(cfg.BaseDir, cfg.DestDir)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBootstrapFailed, err)
	}

	for _, dir := range dirs {
		if err := cfg.createDir(dir); err != nil {
			return fmt.Errorf("%w: %w", ErrBootstrapFailed, err)
		}
	}

	if cfg.Harden != nil {
		if err := cfg.Harden(cfg.DestDir); err != nil {
			return fmt.Errorf("%w: cannot harden %s: %w", ErrBootstrapFailed, cfg.DestDir, err)
		}
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

// createDir creates one component of the path to the config directory, or
// checks it if it turns out somebody else got there first.
func (cfg bootstrapConfig) createDir(dir string) error {
	switch err := os.Mkdir(dir, 0o755); {
	case err == nil:
		// This process created the directory, so this process owns it and
		// there is nothing to check.
		return nil
	case errors.Is(err, os.ErrExist):
		// Somebody else created it. Fall through and find out who.
	default:
		return fmt.Errorf("cannot create %s: %w", dir, err)
	}

	if err := checkPlainDir(dir); err != nil {
		return err
	}

	if cfg.VerifyDir != nil {
		if err := cfg.VerifyDir(dir); err != nil {
			return fmt.Errorf("refusing to use %s: %w", dir, err)
		}
	}

	return nil
}

// checkPlainDir reports whether dir is a real directory rather than a link to
// one.
func checkPlainDir(dir string) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("cannot stat %s: %w", dir, err)
	}

	if fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return fmt.Errorf("%w: %s is a link, not a directory", ErrUntrustedDir, dir)
	}

	if !fi.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrUntrustedDir, dir)
	}

	return nil
}

// dirChain returns every directory from base down to dest, base excluded and
// dest included, in the order they have to be created.
func dirChain(base, dest string) ([]string, error) {
	if base == "" {
		return nil, fmt.Errorf("%w: no base directory given for %s", ErrNotUnderBase, dest)
	}

	rel, err := filepath.Rel(base, dest)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot place %s under %s: %w", ErrNotUnderBase, dest, base, err)
	}

	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("%w: %s is not under %s", ErrNotUnderBase, dest, base)
	}

	var dirs []string
	dir := base
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		dir = filepath.Join(dir, part)
		dirs = append(dirs, dir)
	}

	return dirs, nil
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
