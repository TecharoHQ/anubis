package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// mkmsiRequireVerifyEnv is the opt-in environment variable that turns a skip
// into a failure. It must only be set by workflow jobs that actually install
// msitools and build the zips these tests inspect -- currently
// package-builds-stable.yml and package-builds-unstable.yml's "Verify MSI
// contents" step. Gating on CI alone was tried and failed: GitHub Actions
// sets CI=true on every job, including go.yml, which never installs msitools
// or runs `go tool yeet`, so that made the main Go workflow fail on every PR.
const mkmsiRequireVerifyEnv = "MKMSI_REQUIRE_VERIFY"

// skipOrFail skips the test locally but fails it when mkmsiRequireVerifyEnv
// is set.
//
// These tests are the only automated verification that the MSI's tables
// survived wixl, which silently drops attributes it does not understand. A
// skip and a pass are both exit 0, so a skip in a job that claims to verify
// this would show a green check while checking nothing. Everywhere else --
// including a contributor's laptop and the main Go workflow, neither of
// which installs msitools -- skipping is right.
func skipOrFail(t *testing.T, format string, args ...any) {
	t.Helper()

	if os.Getenv(mkmsiRequireVerifyEnv) == "true" {
		t.Fatalf("refusing to skip with "+mkmsiRequireVerifyEnv+"=true: "+format, args...)
	}

	t.Skipf(format, args...)
}

// requireTool skips the test when a required external program is missing, so
// contributors without msitools installed are not blocked.
func requireTool(t *testing.T, name string) {
	t.Helper()

	if _, err := exec.LookPath(name); err != nil {
		skipOrFail(t, "%s is not installed, skipping: %v", name, err)
	}
}

// findZip returns the Windows zip yeet has built for arch, skipping the test
// if there is none.
//
// It checks both var/ directly and one level of subdirectory: yeet's default
// destination is var/, but the human partner routinely builds with something
// like --package-dest-dir ./var/win.
//
// No zip at all is always a skip, including with mkmsiRequireVerifyEnv set.
// There is nothing to verify without a build, and a checkout that never ran
// `go tool yeet` must not be reported as a broken MSI.
//
// Several candidates for the same architecture are normal on a working
// checkout: var/ accumulates the zips of every release the human partner has
// built. The one that matters is always the newest, so this picks the
// candidate with the latest modification time instead of making the
// contributor delete their own build output to get a green run. Ties break on
// path order, which only happens when two copies were written in the same
// filesystem timestamp tick.
func findZip(t *testing.T, arch string) string {
	t.Helper()

	pattern := "anubis-*-windows-" + arch + ".zip"

	var matches []string
	for _, glob := range []string{
		filepath.Join("..", "..", "..", "var", pattern),
		filepath.Join("..", "..", "..", "var", "*", pattern),
	} {
		found, err := filepath.Glob(glob)
		if err != nil {
			t.Fatalf("Glob: %v", err)
		}
		matches = append(matches, found...)
	}
	sort.Strings(matches)

	switch len(matches) {
	case 0:
		t.Skipf("no Windows %s zip in var or var/*, run `go tool yeet` first", arch)
	case 1:
		return matches[0]
	}

	newest := matches[0]
	newestMod, err := modTime(newest)
	if err != nil {
		t.Fatalf("stat %s: %v", newest, err)
	}

	for _, m := range matches[1:] {
		mod, err := modTime(m)
		if err != nil {
			t.Fatalf("stat %s: %v", m, err)
		}

		if mod.After(newestMod) {
			newest, newestMod = m, mod
		}
	}

	t.Logf("found %d Windows %s zips under var, using the newest: %s (modified %s)", len(matches), arch, newest, newestMod.Format(time.RFC3339))

	return newest
}

// modTime returns the modification time of a file.
func modTime(path string) (time.Time, error) {
	st, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}

	return st.ModTime(), nil
}

// buildTestMSI builds an MSI for arch from the newest zip and returns its
// path.
func buildTestMSI(t *testing.T, arch string) string {
	t.Helper()

	requireTool(t, "wixl")
	requireTool(t, "wixl-heat")
	requireTool(t, "msiinfo")
	// Every arch shells out to msibuild now, to patch SecureCustomProperties
	// after wixl builds the package (see secureInstallDir); arm64 additionally
	// uses it to patch the summary Template.
	requireTool(t, "msibuild")

	out := filepath.Join(t.TempDir(), "anubis-"+arch+".msi")

	cmd := exec.Command("go", "run", ".",
		"--zip", findZip(t, arch),
		"--out", out,
		"--packaging-dir", filepath.Join("..", "..", "..", "run", "windows"),
		"--staging-dir", filepath.Join("..", "..", "..", "var", "mkmsi"),
	)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("mkmsi failed: %v", err)
	}

	return out
}

// exportTable returns one MSI table as text.
//
// msiinfo export writes CRLF line endings. Left alone, the trailing \r rides
// along on whichever column is last in each row -- silently, since it prints
// invisibly and only breaks exact-match comparisons against that column.
// Normalizing here means every caller that splits on "\n" and "\t" gets clean
// fields.
func exportTable(t *testing.T, msi, table string) string {
	t.Helper()

	out, err := exec.Command("msiinfo", "export", msi, table).Output()
	if err != nil {
		t.Fatalf("msiinfo export %s: %v", table, err)
	}

	return strings.ReplaceAll(string(out), "\r\n", "\n")
}

// suminfo returns the value of one field from `msiinfo suminfo`, such as
// "Template" or "Version", with the "Field:" prefix and surrounding
// whitespace stripped.
func suminfo(t *testing.T, msi, field string) string {
	t.Helper()

	out, err := exec.Command("msiinfo", "suminfo", msi).Output()
	if err != nil {
		t.Fatalf("msiinfo suminfo %s: %v", msi, err)
	}

	prefix := field + ":"
	for _, line := range strings.Split(string(out), "\n") {
		if rest, ok := strings.CutPrefix(line, prefix); ok {
			return strings.TrimSpace(rest)
		}
	}

	t.Fatalf("msiinfo suminfo %s has no %q line:\n%s", msi, field, out)
	return ""
}

// suminfoVersion returns the numeric InstallerVersion from `msiinfo
// suminfo`'s Version line, which msitools renders as "200 (c8)".
func suminfoVersion(t *testing.T, msi string) string {
	t.Helper()

	v := suminfo(t, msi, "Version")
	if i := strings.IndexByte(v, ' '); i >= 0 {
		v = v[:i]
	}

	return v
}

// property returns one row's value from the MSI's Property table.
func property(t *testing.T, msi, name string) string {
	t.Helper()

	for _, line := range strings.Split(exportTable(t, msi, "Property"), "\n") {
		fields := strings.SplitN(line, "\t", 2)
		if len(fields) == 2 && fields[0] == name {
			return fields[1]
		}
	}

	t.Fatalf("property %s not found in %s", name, msi)
	return ""
}

// lastTableRow returns the last line of an exported table, which is its
// final data row when the table has exactly one.
func lastTableRow(t *testing.T, msi, table string) string {
	t.Helper()

	lines := strings.Split(strings.TrimRight(exportTable(t, msi, table), "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("table %s in %s is empty", table, msi)
	}

	return lines[len(lines)-1]
}

// componentConditions maps every Component in the MSI to its Condition
// column, which is empty for components that always install.
func componentConditions(t *testing.T, msi string) map[string]string {
	t.Helper()

	conditions := make(map[string]string)

	for _, line := range strings.Split(exportTable(t, msi, "Component"), "\n") {
		// Component  ComponentId  Directory_  Attributes  Condition  KeyPath
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}

		// Attributes is a required integer column; the two header rows
		// (column names, column types) do not parse as one, so this is a
		// clean way to skip them without hardcoding the header text.
		if _, err := strconv.Atoi(fields[3]); err != nil {
			continue
		}

		conditions[fields[0]] = fields[4]
	}

	return conditions
}

func TestMSIContents(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on development machines")
	}

	msi := buildTestMSI(t, "amd64")

	for _, tt := range []struct {
		name  string
		table string
		// want are substrings that must all appear in the exported table.
		want []string
	}{
		{
			name:  "service is registered as manual start under its virtual account",
			table: "ServiceInstall",
			// Columns: ServiceInstall, Name, DisplayName, ServiceType(16 =
			// own process), StartType(3 = manual), ErrorControl(1 = normal).
			//
			// StartName is the NT SERVICE virtual account the service control
			// manager creates alongside the service. It has to be the fully
			// qualified name: a bare "Anubis" or "LocalService" gets looked up
			// as a local user account and the install fails.
			want: []string{"svc_anubis\tAnubis\tAnubis\t16\t3\t1", `NT SERVICE\Anubis`},
		},
		{
			name:  "service is stopped and deleted on uninstall",
			table: "ServiceControl",
			// Event 162 = stop on install, stop and delete on uninstall.
			want: []string{"Anubis\t162"},
		},
		{
			name:  "bin directory is added to the system PATH",
			table: "Environment",
			want:  []string{"=*PATH", "[BINDIR]"},
		},
		{
			name:  "bootstrap custom action is present and deferred",
			table: "CustomAction",
			// 3122 = exe named by a property, deferred, no impersonation.
			want: []string{"SetAnubisExePath\t51", "BootstrapConfig\t3122", "--windows-bootstrap-config"},
		},
		{
			name:  "major upgrades are configured",
			table: "Upgrade",
			want:  []string{"{92F5FC1F-F6F2-4BD1-BBEF-79B8A46C665F}"},
		},
		{
			name:  "both binaries and both config templates are installed",
			table: "File",
			want: []string{
				"anubis.exe",
				"anubis-robots2policy.exe",
				"fil_tmpl_env",
				"fil_tmpl_yaml",
			},
		},
		{
			name:  "the documentation tree is installed",
			table: "File",
			want:  []string{"CHANGELOG.md", "installation.mdx"},
		},
		{
			// Proof the WixUI_Minimal dialog set actually landed in the
			// MSI rather than being silently dropped: wixl has crashed
			// internally on hand-authored dialogs before now and emitted
			// an MSI with no Dialog table at all.
			name:  "the WixUI_Minimal dialog set is present",
			table: "Dialog",
			want:  []string{"WelcomeEulaDlg", "VerifyReadyDlg", "ProgressDlg", "ExitDialog"},
		},
		{
			// secureInstallDir patches this row in after wixl builds the
			// MSI (see its doc comment for why it cannot be done
			// declaratively in anubis.wxs). Without it, a command-line
			// INSTALLDIR= would not survive elevation into the deferred
			// part of the install.
			name:  "INSTALLDIR is a secure custom property",
			table: "Property",
			want:  []string{"SecureCustomProperties\tINSTALLDIR"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := exportTable(t, msi, tt.table)

			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("table %s does not contain %q\ngot:\n%s", tt.table, want, got)
				}
			}
		})
	}
}

// TestMSIStartsServiceOnlyOnUpgrade verifies the fix for an upgrade silently
// leaving Anubis stopped: RemoveExistingProducts sequences before
// InstallServices, so the outgoing product's ServiceControl (Event=162)
// stops and deletes the service, and the incoming ServiceInstall recreates it
// with StartType=demand. Without an explicit start request an upgrade always
// reports success while leaving the site unprotected.
//
// Every ServiceControl row that requests a start (Event bit 0 set) must
// belong to a Component conditioned on WIX_UPGRADE_DETECTED -- the property
// MajorUpgrade sets only when an earlier version of Anubis is already
// installed -- so a fresh install, which has a placeholder target and no
// signing key, never starts the service.
func TestMSIStartsServiceOnlyOnUpgrade(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on development machines")
	}

	msi := buildTestMSI(t, "amd64")

	conditions := componentConditions(t, msi)

	var sawStartRow bool

	for _, line := range strings.Split(exportTable(t, msi, "ServiceControl"), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 6 || fields[1] != "Anubis" {
			continue
		}

		event, err := strconv.Atoi(fields[2])
		if err != nil {
			// Header rows carry column names and types, not numbers.
			continue
		}

		if event&1 == 0 {
			continue
		}

		sawStartRow = true

		component := fields[5]
		if cond := conditions[component]; cond != "WIX_UPGRADE_DETECTED" {
			t.Errorf("ServiceControl row %q requests a start (event %d) via component %q, whose condition is %q, want %q",
				fields[0], event, component, cond, "WIX_UPGRADE_DETECTED")
		}
	}

	if !sawStartRow {
		t.Error("no ServiceControl row for the anubis service requests a start; the upgrade fix appears to be missing")
	}
}

func TestMSIArm64(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on development machines")
	}

	amd64 := buildTestMSI(t, "amd64")
	arm64 := buildTestMSI(t, "arm64")

	t.Run("summary Template names each architecture", func(t *testing.T) {
		if got, want := suminfo(t, amd64, "Template"), "x64;1033"; got != want {
			t.Errorf("amd64 Template = %q, want %q", got, want)
		}
		if got, want := suminfo(t, arm64, "Template"), "Arm64;1033"; got != want {
			t.Errorf("arm64 Template = %q, want %q", got, want)
		}
	})

	t.Run("InstallerVersion is 500 on arm64, 200 on amd64", func(t *testing.T) {
		// Arm64 package templates require Windows Installer 5.0; declaring a
		// lower InstallerVersion risks the MSI being rejected on the target
		// as "not supported by this processor type".
		if got, want := suminfoVersion(t, amd64), "200"; got != want {
			t.Errorf("amd64 Version = %q, want %q", got, want)
		}
		if got, want := suminfoVersion(t, arm64), "500"; got != want {
			t.Errorf("arm64 Version = %q, want %q", got, want)
		}
	})

	t.Run("ProductCode differs between architectures", func(t *testing.T) {
		amdCode := property(t, amd64, "ProductCode")
		armCode := property(t, arm64, "ProductCode")

		if amdCode == armCode {
			t.Errorf("amd64 and arm64 MSIs share ProductCode %q; Windows would treat installing one as uninstalling the other", amdCode)
		}
	})

	t.Run("ServiceInstall is identical across architectures", func(t *testing.T) {
		amdSvc := lastTableRow(t, amd64, "ServiceInstall")
		armSvc := lastTableRow(t, arm64, "ServiceInstall")

		if amdSvc != armSvc {
			t.Errorf("ServiceInstall row differs between architectures:\namd64: %s\narm64: %s", amdSvc, armSvc)
		}
	})
}

// extractStream returns the raw bytes of one binary stream from an MSI, as
// reported by `msiinfo extract`.
func extractStream(t *testing.T, msi, stream string) []byte {
	t.Helper()

	out, err := exec.Command("msiinfo", "extract", msi, stream).Output()
	if err != nil {
		t.Fatalf("msiinfo extract %s %s: %v", msi, stream, err)
	}

	return out
}

// TestMSIInstallerImages verifies patchInstallerImages actually replaced
// wixl's stock WiX banner and dialog artwork with run/windows/banner.bmp and
// run/windows/dialog.bmp, rather than the msibuild -a calls silently no-oping
// the way Property/@Secure="yes" did earlier in this file's history. Byte
// length is a cheap, robust stand-in for a full comparison: it is exactly
// what changes if someone swaps in a real image and the patch stops running,
// since the stock artwork's size is fixed and different from Anubis's.
func TestMSIInstallerImages(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on development machines")
	}

	msi := buildTestMSI(t, "amd64")

	for _, img := range installerImages {
		t.Run(img.stream, func(t *testing.T) {
			source := filepath.Join("..", "..", "..", "run", "windows", img.file)

			wantInfo, err := os.Stat(source)
			if err != nil {
				t.Fatalf("stat %s: %v", source, err)
			}

			got := extractStream(t, msi, img.stream)

			if int64(len(got)) != wantInfo.Size() {
				t.Errorf("stream %s is %d bytes, %s is %d bytes -- patchInstallerImages may not have run",
					img.stream, len(got), source, wantInfo.Size())
			}
		})
	}
}

// rtfControlWord matches one RTF control word, with its optional numeric
// parameter and the single trailing space that terminates it, such as
// `\ansicpg1252 ` or `\par`.
var rtfControlWord = regexp.MustCompile(`\\[a-zA-Z]+-?[0-9]*[ ]?`)

// stripRTF reduces an RTF document to plain text: control words (\rtf1,
// \par, \f0, ...) and group braces are removed, and the rest -- including
// stray literal text from groups like the font table, which this does not
// try to identify -- is collapsed to single-spaced words.
//
// This is deliberately crude rather than a real RTF parser. It only needs to
// let TestMSILicenseRTFMatchesLicense find the license wording inside
// License.rtf; a byte-exact comparison would make that test brittle for no
// benefit.
func stripRTF(s string) string {
	s = rtfControlWord.ReplaceAllString(s, " ")
	s = strings.NewReplacer("{", " ", "}", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// TestMSILicenseRTFMatchesLicense fails if run/windows/License.rtf, which
// WixUI_Minimal's EULA dialog displays, has fallen out of sync with the
// repo's LICENSE. Someone editing LICENSE has no other reason to think about
// the installer, so nothing else catches this.
func TestMSILicenseRTFMatchesLicense(t *testing.T) {
	license, err := os.ReadFile(filepath.Join("..", "..", "..", "LICENSE"))
	if err != nil {
		t.Fatalf("reading LICENSE: %v", err)
	}

	rtf, err := os.ReadFile(filepath.Join("..", "..", "..", "run", "windows", "License.rtf"))
	if err != nil {
		t.Fatalf("reading run/windows/License.rtf: %v", err)
	}

	if !bytes.HasPrefix(rtf, []byte(`{\rtf1`)) {
		t.Fatalf("run/windows/License.rtf does not start with %q, is it valid RTF?", `{\rtf1`)
	}

	rtfText := stripRTF(string(rtf))

	for _, para := range strings.Split(string(license), "\n\n") {
		// LICENSE hard-wraps its paragraphs; License.rtf does not, so compare
		// each paragraph as one line.
		para = strings.Join(strings.Fields(para), " ")
		if para == "" {
			continue
		}

		if !strings.Contains(rtfText, para) {
			t.Errorf("run/windows/License.rtf is missing text from LICENSE, or has fallen out of sync with it\nmissing paragraph: %q", para)
		}
	}
}

// msiTables returns the sorted list of table names in an MSI, as reported by
// `msiinfo tables`.
func msiTables(t *testing.T, msi string) []string {
	t.Helper()

	out, err := exec.Command("msiinfo", "tables", msi).Output()
	if err != nil {
		t.Fatalf("msiinfo tables %s: %v", msi, err)
	}

	var tables []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			tables = append(tables, line)
		}
	}
	sort.Strings(tables)

	return tables
}

// msiStreams returns the sorted list of stream names in an MSI, as reported
// by `msiinfo streams`.
func msiStreams(t *testing.T, msi string) []string {
	t.Helper()

	out, err := exec.Command("msiinfo", "streams", msi).Output()
	if err != nil {
		t.Fatalf("msiinfo streams %s: %v", msi, err)
	}

	var streams []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			streams = append(streams, line)
		}
	}
	sort.Strings(streams)

	return streams
}

// sortedLines splits s on newlines and sorts the result, so two exports of
// the same table can be compared as sets when row order is not guaranteed
// to match.
func sortedLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	sort.Strings(lines)
	return lines
}

// diffByteCount returns how many bytes differ between a and b at the same
// offset, plus any length mismatch, for a more informative log message than
// a bare "not equal" when two builds turn out not to be byte-identical.
func diffByteCount(a, b []byte) int {
	n := 0
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			n++
		}
	}
	if len(a) > len(b) {
		n += len(a) - len(b)
	} else {
		n += len(b) - len(a)
	}
	return n
}

// requireSuminfoMatchesExceptTimestamps fails the test if two MSIs' summary
// information differs anywhere other than the Created and Last saved
// fields. wixl stamps both from the wall clock when it builds the package,
// and msibuild's -s -- the only tool available to rewrite summary
// information -- has no argument for either one; see
// TestMSIReproducibleBuild's doc comment.
func requireSuminfoMatchesExceptTimestamps(t *testing.T, msi1, msi2 string) {
	t.Helper()

	out1, err := exec.Command("msiinfo", "suminfo", msi1).Output()
	if err != nil {
		t.Fatalf("msiinfo suminfo %s: %v", msi1, err)
	}
	out2, err := exec.Command("msiinfo", "suminfo", msi2).Output()
	if err != nil {
		t.Fatalf("msiinfo suminfo %s: %v", msi2, err)
	}

	lines1 := strings.Split(strings.TrimRight(string(out1), "\n"), "\n")
	lines2 := strings.Split(strings.TrimRight(string(out2), "\n"), "\n")

	if len(lines1) != len(lines2) {
		t.Fatalf("suminfo line count differs between %s (%d) and %s (%d):\n%s\nvs\n%s",
			msi1, len(lines1), msi2, len(lines2), out1, out2)
	}

	for i := range lines1 {
		if lines1[i] == lines2[i] {
			continue
		}
		if strings.HasPrefix(lines1[i], "Created:") || strings.HasPrefix(lines1[i], "Last saved:") {
			continue
		}
		t.Errorf("summary information differs beyond the known-unfixable Created/Last saved timestamps:\n%q\nvs\n%q", lines1[i], lines2[i])
	}
}

// TestMSIReproducibleBuild builds the same zip into two separate MSIs and
// asserts the results match, aside from two residuals that could not be
// removed with the tools available (msitools' wixl, wixl-heat and
// msibuild):
//
//   - The summary information's Created and Last saved timestamps: wixl
//     stamps both from the wall clock when it builds the package, and
//     msibuild's -s -- the only available way to rewrite summary
//     information -- has no argument for either one.
//   - The InstallExecuteSequence table's on-disk row order for
//     SetAnubisExePath and BootstrapConfig. patchCustomActionSequence pins
//     their Sequence *values* deterministically, but not their physical
//     storage position: deleting a row and reinserting it under the same
//     Action was observed, empirically, to always land the row back in its
//     original slot, so nothing reachable through msibuild's SQL interface
//     can move it. Windows Installer executes actions in Sequence order
//     regardless of on-disk row order, so this has no effect on how the
//     installer behaves -- see patchCustomActionSequence's doc comment for
//     the full account.
//
// Everything else -- every other table, and every stream including the CAB
// archive -- is required to be exactly byte-identical.
func TestMSIReproducibleBuild(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on development machines")
	}

	requireTool(t, "wixl")
	requireTool(t, "wixl-heat")
	requireTool(t, "msiinfo")
	requireTool(t, "msibuild")

	zip := findZip(t, "amd64")

	build := func(out string) {
		t.Helper()

		cmd := exec.Command("go", "run", ".",
			"--zip", zip,
			"--out", out,
			"--packaging-dir", filepath.Join("..", "..", "..", "run", "windows"),
			"--staging-dir", filepath.Join("..", "..", "..", "var", "mkmsi"),
		)
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			t.Fatalf("mkmsi failed building %s: %v", out, err)
		}
	}

	out1 := filepath.Join(t.TempDir(), "build1.msi")
	out2 := filepath.Join(t.TempDir(), "build2.msi")
	build(out1)
	build(out2)

	data1, err := os.ReadFile(out1)
	if err != nil {
		t.Fatalf("reading %s: %v", out1, err)
	}
	data2, err := os.ReadFile(out2)
	if err != nil {
		t.Fatalf("reading %s: %v", out2, err)
	}

	if bytes.Equal(data1, data2) {
		// Byte-identical: every check below would necessarily pass too.
		return
	}

	t.Logf("%s and %s are not byte-identical (%d of %d bytes differ); checking whether the difference is limited to the known residuals",
		out1, out2, diffByteCount(data1, data2), max(len(data1), len(data2)))

	requireSuminfoMatchesExceptTimestamps(t, out1, out2)

	tables1 := msiTables(t, out1)
	tables2 := msiTables(t, out2)
	if !slices.Equal(tables1, tables2) {
		t.Fatalf("table lists differ:\nbuild1: %v\nbuild2: %v", tables1, tables2)
	}

	for _, table := range tables1 {
		switch table {
		case "_SummaryInformation":
			// Already checked, field by field, via `msiinfo suminfo`
			// above -- more precisely than a raw property-stream export
			// could, since it can name exactly which field differs.
			continue

		case "InstallExecuteSequence":
			// Row order in this one table is a known residual; compare it
			// as a set of rows instead of as ordered text. If the values
			// themselves differ, that is a real bug in
			// patchCustomActionSequence, not a tolerated residual.
			rows1 := sortedLines(exportTable(t, out1, table))
			rows2 := sortedLines(exportTable(t, out2, table))
			if !slices.Equal(rows1, rows2) {
				t.Errorf("table %s differs even as a set of rows (this would mean the Sequence *values* differ, not just row order):\nbuild1: %v\nbuild2: %v",
					table, rows1, rows2)
			}
			continue
		}

		if got, want := exportTable(t, out1, table), exportTable(t, out2, table); got != want {
			t.Errorf("table %s differs:\n--- %s ---\n%s\n--- %s ---\n%s", table, out1, got, out2, want)
		}
	}

	streams1 := msiStreams(t, out1)
	streams2 := msiStreams(t, out2)
	if !slices.Equal(streams1, streams2) {
		t.Fatalf("stream lists differ:\nbuild1: %v\nbuild2: %v", streams1, streams2)
	}

	for _, stream := range streams1 {
		// OLE marks a property-set stream by prefixing its name with 0x05;
		// `msiinfo streams` prints that literally, so this is
		// "\x05SummaryInformation", not "SummaryInformation".
		if strings.HasSuffix(stream, "SummaryInformation") {
			// The raw property-set stream backing both the
			// _SummaryInformation table and `msiinfo suminfo` -- already
			// checked field by field, with the Created/Last saved
			// exception, by requireSuminfoMatchesExceptTimestamps above.
			continue
		}

		a := extractStream(t, out1, stream)
		b := extractStream(t, out2, stream)
		if !bytes.Equal(a, b) {
			t.Errorf("stream %s differs: %d bytes in build1, %d bytes in build2", stream, len(a), len(b))
		}
	}
}
