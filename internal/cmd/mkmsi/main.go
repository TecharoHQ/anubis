// Command mkmsi turns the Windows zip that yeet builds into an MSI installer.
//
// It needs wixl, wixl-heat, msibuild and msiinfo from msitools on PATH.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

var (
	zipPath      = flag.String("zip", "", "path to the Windows zip built by yeet")
	outPath      = flag.String("out", "", "path to write the MSI to")
	packagingDir = flag.String("packaging-dir", filepath.Join("run", "windows"), "directory holding anubis.wxs and the config templates")
	stagingDir   = flag.String("staging-dir", filepath.Join("var", "mkmsi"), "directory to stage the MSI build in; an <arch> subdirectory is used and its previous contents are removed first")
)

// The installer is 64-bit only. anubis.wxs installs into ProgramFiles64Folder
// and marks its components Win64, so there is no meaningful 32-bit build to
// select. An --arch flag here would only let a caller produce an MSI that
// claims to be 32-bit while installing 64-bit components.
const (
	msiArch  = "x64"
	msiWin64 = "yes"

	// msiInstallerVersionAmd64 declares the Windows Installer engine version
	// the package requires. 200 is the long-standing baseline and is what
	// amd64 has always shipped.
	msiInstallerVersionAmd64 = "200"
	// msiInstallerVersionArm64 must be 500: the Arm64 package template (see
	// setArm64Template) was only introduced in Windows Installer 5.0. An MSI
	// declaring a lower InstallerVersion alongside an Arm64 template risks
	// being rejected on the target as "not supported by this processor
	// type" instead of failing at build time.
	msiInstallerVersionArm64 = "500"
)

func main() {
	flag.Parse()

	if *zipPath == "" || *outPath == "" {
		log.Fatal("both --zip and --out are required")
	}

	if err := build(); err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Println(*outPath)
}

func build() error {
	version, err := versionFromZipName(*zipPath)
	if err != nil {
		return err
	}

	arch, err := archFromZipName(*zipPath)
	if err != nil {
		return err
	}

	msiVer, err := msiVersion(version)
	if err != nil {
		return err
	}

	code, err := productCode(version, arch)
	if err != nil {
		return err
	}

	// Computed here, from the raw version string, rather than inside
	// runWixl/patchSummaryInfo: those only ever see msiVer, the already-
	// converted numeric x.y.z.w form, which msiVersion (and therefore
	// packageCode) cannot parse back.
	pkgCode, err := packageCode(version, arch)
	if err != nil {
		return err
	}

	// Staged per architecture so an amd64 and an arm64 build running one
	// after the other -- as both branches of buildTestMSI do -- cannot
	// contaminate each other's tree.
	staging := filepath.Join(*stagingDir, arch)

	// Wipe any previous run's contents before staging fresh ones. A leftover
	// file from an earlier build would otherwise still be sitting in
	// staging and silently end up in this MSI. The tree is intentionally
	// left in place after a successful build (no deferred cleanup) so it
	// can be inspected -- var/mkmsi already only lives under a gitignored
	// directory, and being able to diff two builds' staging trees directly
	// is worth more than tidying up after every run.
	if err := os.RemoveAll(staging); err != nil {
		return fmt.Errorf("mkmsi: cannot clear staging directory %s: %w", staging, err)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return fmt.Errorf("mkmsi: cannot create staging directory %s: %w", staging, err)
	}

	_, srcMTime, err := unzip(*zipPath, staging)
	if err != nil {
		return err
	}

	if err := stageConfigTemplates(staging, srcMTime); err != nil {
		return err
	}

	docWxs := filepath.Join(staging, "doc-files.wxs")
	if err := generateDocFragment(staging, docWxs); err != nil {
		return err
	}

	return runWixl(staging, docWxs, msiVer, code, pkgCode, arch)
}

// stageConfigTemplates builds the etc directory the installer ships: the env
// file as written, and the default policy with the Windows logging block
// appended. mtime is stamped on both, so they carry the same reproducible
// mtime as everything unzip extracted from the source zip; see concatFiles.
func stageConfigTemplates(staging string, mtime time.Time) error {
	etc := filepath.Join(staging, "etc")

	if err := concatFiles(filepath.Join(etc, "anubis.env"), mtime, filepath.Join(*packagingDir, "anubis.env")); err != nil {
		return err
	}

	policy := filepath.Join(staging, "doc", "botPolicies.yaml")
	if err := concatFiles(filepath.Join(etc, "anubis.yaml"), mtime, policy, filepath.Join(*packagingDir, "logging.yaml")); err != nil {
		return err
	}

	return nil
}

// generateDocFragment runs wixl-heat over the documentation tree. The file
// list is sorted so the fragment, and therefore the MSI, is reproducible.
func generateDocFragment(staging, out string) error {
	docDir := filepath.Join(staging, "doc")

	var files []string
	err := filepath.WalkDir(docDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("mkmsi: cannot walk %s: %w", docDir, err)
	}

	slices.Sort(files)

	// Root the fragment at DOCDIR, which anubis.wxs already declares, and give
	// it its own source variable. Pointing wixl-heat at INSTALLDIR instead
	// would make it emit a second "doc" directory alongside the declared one.
	cmd := exec.Command("wixl-heat",
		"--prefix", docDir+string(os.PathSeparator),
		"--directory-ref", "DOCDIR",
		"--component-group", "DocFiles",
		"--var", "var.DocSourceDir",
		"--win64",
	)
	cmd.Stdin = strings.NewReader(strings.Join(files, "\n") + "\n")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	// Check stderr before the exit status, exactly as runWixl does. wixl
	// reports a dropped attribute this way and then exits 0, so the exit
	// status alone cannot be trusted, and a non-zero exit accompanied by a
	// dropped attribute must still surface as ErrWixlDroppedAttribute.
	if err := checkWixlOutput(stderr.String()); err != nil {
		return err
	}

	if runErr != nil {
		return fmt.Errorf("mkmsi: wixl-heat failed: %w: %s", runErr, stderr.String())
	}

	// wixl-heat randomizes its Directory IDs on every invocation, even over
	// an identical tree; see rewriteDirectoryIDs for the details and why
	// that alone breaks build reproducibility.
	fragment, err := rewriteDirectoryIDs(stdout.Bytes())
	if err != nil {
		return err
	}

	if err := os.WriteFile(out, fragment, 0o644); err != nil {
		return fmt.Errorf("mkmsi: cannot write %s: %w", out, err)
	}

	return nil
}

// runWixl compiles the wxs sources into the output MSI. version and code are
// the MSI ProductVersion and ProductCode (msiVersion/productCode's output);
// pkgCode is the PackageCode patched in afterward (packageCode's output,
// still the raw UUID, not yet braced/uppercased).
func runWixl(staging, docWxs, version, code, pkgCode, arch string) error {
	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		return fmt.Errorf("mkmsi: cannot create %s: %w", filepath.Dir(*outPath), err)
	}

	installerVersion := msiInstallerVersionAmd64
	if arch == "arm64" {
		installerVersion = msiInstallerVersionArm64
	}

	cmd := exec.Command("wixl",
		"-a", msiArch,
		"--ext", "ui",
		"-D", "SourceDir="+staging,
		"-D", "DocSourceDir="+filepath.Join(staging, "doc"),
		"-D", "Win64="+msiWin64,
		"-D", "Version="+version,
		"-D", "ProductCode="+code,
		"-D", "InstallerVersion="+installerVersion,
		"-o", *outPath,
		filepath.Join(*packagingDir, "anubis.wxs"),
		docWxs,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = os.Stdout

	runErr := cmd.Run()

	// Check stderr first: wixl reports a dropped attribute this way and then
	// exits 0, which would otherwise look like success.
	if err := checkWixlOutput(stderr.String()); err != nil {
		return err
	}

	if runErr != nil {
		return fmt.Errorf("mkmsi: wixl failed: %w: %s", runErr, stderr.String())
	}

	if err := secureInstallDir(*outPath); err != nil {
		return err
	}

	if err := patchInstallerImages(*outPath); err != nil {
		return err
	}

	if err := patchSummaryInfo(*outPath, pkgCode, arch); err != nil {
		return err
	}

	if err := patchEnvironmentID(*outPath); err != nil {
		return err
	}

	if err := patchCustomActionSequence(*outPath); err != nil {
		return err
	}

	return nil
}

// installerImage pairs one wixl UI extension Binary stream with the image in
// *packagingDir that replaces it.
type installerImage struct {
	stream string
	file   string
}

// installerImages lists the streams patchInstallerImages replaces.
// WixUI_Bmp_Banner is the strip shown across the top of every wizard page;
// WixUI_Bmp_Dialog is the full-height image on the welcome and exit pages.
//
// This is a slice, not a map, so the msibuild arguments below are built in a
// fixed order: build outputs should be reproducible, and map iteration order
// is not.
var installerImages = []installerImage{
	{"Binary.WixUI_Bmp_Banner", "banner.bmp"},
	{"Binary.WixUI_Bmp_Dialog", "dialog.bmp"},
}

// patchInstallerImages replaces wixl's stock WiX banner and dialog artwork
// with Anubis's own, for both architectures.
//
// There is no declarative way to do this: <WixVariable Id="WixUIBannerBmp">,
// how real WiX overrides these, is not supported by wixl. The alternative --
// pointing --extdir at a local copy of wixl's ext/ui tree so the Binary
// SourceFile paths can be edited -- would mean vendoring ~18 MS-RL licensed
// .wxs files into this MIT-licensed repo, so this patches the built MSI's
// Binary streams directly instead, the same way secureInstallDir and
// setArm64Template patch other parts of the file after wixl runs.
func patchInstallerImages(msiPath string) error {
	args := []string{msiPath}
	for _, img := range installerImages {
		args = append(args, "-a", img.stream, filepath.Join(*packagingDir, img.file))
	}

	cmd := exec.Command("msibuild", args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkmsi: msibuild failed to patch installer images: %w: %s", err, stderr.String())
	}

	// msibuild's exit status is not proof either stream actually changed --
	// see secureInstallDir and setArm64Template for why that trust has
	// already twice proven misplaced in this file. Extract each stream back
	// and compare its length against the source image rather than assume
	// the write landed.
	for _, img := range installerImages {
		source := filepath.Join(*packagingDir, img.file)

		wantInfo, err := os.Stat(source)
		if err != nil {
			return fmt.Errorf("mkmsi: cannot stat %s: %w", source, err)
		}

		got, err := exec.Command("msiinfo", "extract", msiPath, img.stream).Output()
		if err != nil {
			return fmt.Errorf("mkmsi: msiinfo extract %s %s failed: %w", msiPath, img.stream, err)
		}

		if int64(len(got)) != wantInfo.Size() {
			return fmt.Errorf("mkmsi: msibuild did not patch %s on %s: stream is %d bytes, %s is %d bytes",
				img.stream, msiPath, len(got), source, wantInfo.Size())
		}
	}

	return nil
}

// secureInstallDir marks INSTALLDIR as a secure custom property, so that a
// command-line `msiexec /i anubis.msi INSTALLDIR=...` override survives
// elevation into the deferred, system-context part of the install. Without
// it, the override is silently ignored and Anubis installs to the default
// location anyway.
//
// This can't be done declaratively in anubis.wxs. Both obvious forms fail
// under wixl 0.106:
//
//   - Property/@Secure="yes" is silently dropped; no SecureCustomProperties
//     row results at all.
//   - A hand-authored <Property Id="SecureCustomProperties"> collides with
//     the row wixl generates automatically for MajorUpgrade (populated with
//     WIX_UPGRADE_DETECTED and WIX_DOWNGRADE_DETECTED) and aborts the wixl
//     build outright with a bare "libmsi_query_execute" and no further
//     explanation.
//
// So, the same way setArm64Template patches the summary Template after the
// fact, this patches the SecureCustomProperties row after wixl has built it.
func secureInstallDir(msiPath string) error {
	existing, err := readProperty(msiPath, "SecureCustomProperties")
	if err != nil {
		return err
	}

	if slices.Contains(strings.Split(existing, ";"), "INSTALLDIR") {
		return nil
	}

	want := "INSTALLDIR"
	if existing != "" {
		want += ";" + existing
	}

	// This assumes the SecureCustomProperties row already exists, which
	// MajorUpgrade above guarantees today. If that ever stops being true,
	// this UPDATE silently affects zero rows -- but the read-back below
	// still catches it and fails loudly rather than shipping an MSI where
	// INSTALLDIR= quietly does nothing.
	cmd := exec.Command("msibuild", msiPath, "-q",
		fmt.Sprintf("UPDATE Property SET Value='%s' WHERE Property='SecureCustomProperties'", want))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkmsi: msibuild failed to secure INSTALLDIR: %w: %s", err, stderr.String())
	}

	got, err := readProperty(msiPath, "SecureCustomProperties")
	if err != nil {
		return err
	}

	if !slices.Contains(strings.Split(got, ";"), "INSTALLDIR") {
		return fmt.Errorf("mkmsi: msibuild did not add INSTALLDIR to SecureCustomProperties on %s: got %q", msiPath, got)
	}

	return nil
}

// readProperty returns one row's value from an MSI's Property table, as
// reported by `msiinfo export`, or "" if the property has no row.
func readProperty(msiPath, name string) (string, error) {
	out, err := exec.Command("msiinfo", "export", msiPath, "Property").Output()
	if err != nil {
		return "", fmt.Errorf("mkmsi: msiinfo export %s Property failed: %w", msiPath, err)
	}

	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		if prop, value, ok := strings.Cut(line, "\t"); ok && prop == name {
			return value, nil
		}
	}

	return "", nil
}

// patchSummaryInfo rewrites the summary information stream after wixl has
// built the MSI: the Template field, and the PackageCode (the summary's
// "Revision number (UUID)" field).
//
// Both go through one msibuild -s call site on purpose. msibuild -s
// overwrites Subject, Template and PackageCode together every time it is
// called -- there is no way to set one without restating the other two --
// so a second call site (there used to be one here, setting only Template
// for arm64) would silently clobber whatever this one had just written, or
// vice versa depending on call order. One call site that always states all
// three is the only way they cannot fight.
//
// wixl 0.106 has no arm64 target — it accepts only intel, intel64 and ia64 —
// so an arm64 package is built as x64 and its Template corrected here. Only
// the declared architecture is wrong; the component bitness and the
// ProgramFiles64Folder install location are already right for arm64.
//
// PackageCode is set to a deterministic UUIDv5 (see packageCode) instead of
// the random value wixl assigns on every build; see packageCode's doc
// comment for why turning off that randomness is correct once the rest of
// the build is already reproducible. pkgCode is packageCode's raw output,
// formatted here to match the braced, uppercase shape wixl itself writes
// ProductCode and UpgradeCode in.
func patchSummaryInfo(msiPath, pkgCode, arch string) error {
	template := "x64;1033"
	if arch == "arm64" {
		template = "Arm64;1033"
	}

	wantCode := "{" + strings.ToUpper(pkgCode) + "}"

	cmd := exec.Command("msibuild", msiPath,
		"-s", "Anubis Web AI Firewall Utility", "Techaro", template, wantCode,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkmsi: msibuild failed to set summary information: %w: %s", err, stderr.String())
	}

	// msibuild's exit status alone is not proof either field was actually
	// changed: this project has already learned wixl exits 0 while silently
	// dropping data, and msibuild deserves no more trust. Read both back and
	// fail loudly rather than shipping an MSI with the wrong Template, or a
	// PackageCode nobody can reproduce.
	gotTemplate, err := readSummaryTemplate(msiPath)
	if err != nil {
		return err
	}
	if gotTemplate != template {
		return fmt.Errorf("mkmsi: msibuild did not set the summary template on %s: got %q, want %q", msiPath, gotTemplate, template)
	}

	gotCode, err := readSummaryPackageCode(msiPath)
	if err != nil {
		return err
	}
	if gotCode != wantCode {
		return fmt.Errorf("mkmsi: msibuild did not set the PackageCode on %s: got %q, want %q", msiPath, gotCode, wantCode)
	}

	return nil
}

// readSummaryTemplate returns the Template field from an MSI's summary
// information stream, as reported by `msiinfo suminfo`.
func readSummaryTemplate(msiPath string) (string, error) {
	out, err := exec.Command("msiinfo", "suminfo", msiPath).Output()
	if err != nil {
		return "", fmt.Errorf("mkmsi: msiinfo suminfo %s failed: %w", msiPath, err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, "Template:"); ok {
			return strings.TrimSpace(rest), nil
		}
	}

	return "", fmt.Errorf("mkmsi: msiinfo suminfo %s has no Template line", msiPath)
}

// readSummaryPackageCode returns the "Revision number (UUID)" field -- the
// PackageCode -- from an MSI's summary information stream, as reported by
// `msiinfo suminfo`.
func readSummaryPackageCode(msiPath string) (string, error) {
	out, err := exec.Command("msiinfo", "suminfo", msiPath).Output()
	if err != nil {
		return "", fmt.Errorf("mkmsi: msiinfo suminfo %s failed: %w", msiPath, err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, "Revision number (UUID):"); ok {
			return strings.TrimSpace(rest), nil
		}
	}

	return "", fmt.Errorf("mkmsi: msiinfo suminfo %s has no Revision number (UUID) line", msiPath)
}

// patchEnvironmentID overwrites the Environment table's primary key for the
// PATH row with a fixed value.
//
// wixl 0.106 ignores the Id="env_path" declared in anubis.wxs for this row
// and assigns a random GUID to the Environment table's primary key column
// instead, so the row's key differs on every otherwise-identical build.
//
// This can't reuse secureInstallDir's UPDATE approach: Environment is the
// primary key of this table, and the Windows Installer SQL dialect
// (verified empirically -- msibuild reports "failed to execute query" and
// libmsi logs a CRITICAL on the attempt) refuses to UPDATE a primary key
// column. Deleting the row and re-inserting it with the same Name and Value
// but a fixed key is the documented way around that.
func patchEnvironmentID(msiPath string) error {
	const (
		component = "cmp_path"
		want      = "env_path"
	)

	name, value, err := readEnvironmentRow(msiPath, component)
	if err != nil {
		return err
	}

	cmd := exec.Command("msibuild", msiPath, "-q",
		fmt.Sprintf("DELETE FROM Environment WHERE Component_='%s'", sqlEscape(component)),
		"-q",
		fmt.Sprintf("INSERT INTO Environment (Environment, Name, Value, Component_) VALUES ('%s', '%s', '%s', '%s')",
			sqlEscape(want), sqlEscape(name), sqlEscape(value), sqlEscape(component)),
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkmsi: msibuild failed to patch the Environment table: %w: %s", err, stderr.String())
	}

	gotID, gotName, gotValue, err := readEnvironmentRowByID(msiPath, want)
	if err != nil {
		return err
	}

	if gotID != want || gotName != name || gotValue != value {
		return fmt.Errorf("mkmsi: msibuild did not replace the Environment row for %s on %s correctly: got (%q, %q, %q), want (%q, %q, %q)",
			component, msiPath, gotID, gotName, gotValue, want, name, value)
	}

	return nil
}

// sqlEscape doubles single quotes so a value can be embedded in an MSI SQL
// string literal. None of the values this program passes through it are
// expected to contain one -- Name and Value come from anubis.wxs, which
// this project controls -- but the values still flow through here rather
// than straight into a query string, the same discipline this being
// security-sensitive software calls for anywhere user- or file-sourced text
// reaches a query.
func sqlEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// readEnvironmentRow returns the Name and Value columns of the Environment
// table's row for component, as reported by `msiinfo export`.
func readEnvironmentRow(msiPath, component string) (name, value string, err error) {
	out, err := exec.Command("msiinfo", "export", msiPath, "Environment").Output()
	if err != nil {
		return "", "", fmt.Errorf("mkmsi: msiinfo export %s Environment failed: %w", msiPath, err)
	}

	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		// Environment  Name  Value  Component_
		fields := strings.Split(line, "\t")
		if len(fields) == 4 && fields[3] == component {
			return fields[1], fields[2], nil
		}
	}

	return "", "", fmt.Errorf("mkmsi: msiinfo export %s Environment has no row for component %s", msiPath, component)
}

// readEnvironmentRowByID returns the Name and Value columns of the
// Environment table's row keyed by id, as reported by `msiinfo export`.
func readEnvironmentRowByID(msiPath, id string) (gotID, name, value string, err error) {
	out, err := exec.Command("msiinfo", "export", msiPath, "Environment").Output()
	if err != nil {
		return "", "", "", fmt.Errorf("mkmsi: msiinfo export %s Environment failed: %w", msiPath, err)
	}

	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		// Environment  Name  Value  Component_
		fields := strings.Split(line, "\t")
		if len(fields) == 4 && fields[0] == id {
			return fields[0], fields[1], fields[2], nil
		}
	}

	return "", "", "", fmt.Errorf("mkmsi: msiinfo export %s Environment has no row with key %s", msiPath, id)
}

// patchCustomActionSequence pins the InstallExecuteSequence Sequence values
// of SetAnubisExePath and BootstrapConfig -- the two custom actions
// anubis.wxs schedules only relative to each other and to InstallFiles
// ("Before BootstrapConfig" and "After InstallFiles" respectively), never to
// an absolute Sequence number of their own.
//
// This is not one of the five sources of drift this file was written to
// fix; it was found while verifying them. wixl resolves relative
// Before/After hints into absolute Sequence numbers itself, and that
// resolution is not deterministic: across repeated builds of
// byte-identical staged input, it has been observed (empirically, running
// the same build eight times in a row) to place SetAnubisExePath at either
// 1402 (immediately after RemoveExistingProducts) or 4001 (immediately
// after InstallFiles, immediately before BootstrapConfig), with
// BootstrapConfig following one step behind either way. Both placements
// satisfy the wxs's own constraints, so wixl's exit status gives no signal
// anything is wrong.
//
// The tightly-packed placement is the one this pins to, because it is the
// only one actually implied by the wxs text: nothing in anubis.wxs
// schedules SetAnubisExePath relative to RemoveExistingProducts, so pinning
// to that position would invent an ordering guarantee the wxs never asked
// for. InstallFiles's own Sequence is read back rather than hardcoded, so
// this keeps working if its position ever changes.
//
// This corrects the value only, not the table's on-disk row order, which
// was tried and abandoned: deleting a row and reinserting it under the same
// Action was observed, empirically, to always land the row back in its
// original physical slot regardless of when or in what order the insert
// ran -- slot placement in this MSI table format appears to be a function
// of the Action string alone, immutable from outside once wixl first
// writes the row, and not reachable through msibuild's SQL interface. Since
// Windows Installer executes actions in Sequence order regardless of
// on-disk row order, that residual is cosmetic -- but it does mean two
// otherwise byte-identical MSIs can still differ in exactly where this
// table's rows for SetAnubisExePath and BootstrapConfig physically sit, in
// addition to the summary information timestamps. TestMSIReproducibleBuild
// tolerates both.
func patchCustomActionSequence(msiPath string) error {
	installFiles, err := readSequenceNumber(msiPath, "InstallFiles")
	if err != nil {
		return err
	}

	// Applied in this fixed order, not map iteration, so the msibuild
	// invocations themselves are reproducible.
	type pin struct {
		action string
		want   int
	}
	pins := []pin{
		{"SetAnubisExePath", installFiles + 1},
		{"BootstrapConfig", installFiles + 2},
	}

	for _, p := range pins {
		cmd := exec.Command("msibuild", msiPath, "-q",
			fmt.Sprintf("UPDATE InstallExecuteSequence SET Sequence=%d WHERE Action='%s'", p.want, sqlEscape(p.action)))

		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		cmd.Stdout = os.Stdout

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("mkmsi: msibuild failed to pin the sequence for %s: %w: %s", p.action, err, stderr.String())
		}
	}

	for _, p := range pins {
		got, err := readSequenceNumber(msiPath, p.action)
		if err != nil {
			return err
		}
		if got != p.want {
			return fmt.Errorf("mkmsi: msibuild did not pin the sequence for %s on %s: got %d, want %d", p.action, msiPath, got, p.want)
		}
	}

	return nil
}

// readSequenceNumber returns the Sequence column for one Action row of the
// InstallExecuteSequence table, as reported by `msiinfo export`.
func readSequenceNumber(msiPath, action string) (int, error) {
	out, err := exec.Command("msiinfo", "export", msiPath, "InstallExecuteSequence").Output()
	if err != nil {
		return 0, fmt.Errorf("mkmsi: msiinfo export %s InstallExecuteSequence failed: %w", msiPath, err)
	}

	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		// Action  Condition  Sequence
		fields := strings.Split(line, "\t")
		if len(fields) == 3 && fields[0] == action {
			seq, err := strconv.Atoi(fields[2])
			if err != nil {
				return 0, fmt.Errorf("mkmsi: InstallExecuteSequence row for %s in %s has a non-numeric Sequence %q: %w", action, msiPath, fields[2], err)
			}
			return seq, nil
		}
	}

	return 0, fmt.Errorf("mkmsi: msiinfo export %s InstallExecuteSequence has no row for action %s", msiPath, action)
}
