//go:build windows

package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/joho/godotenv"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
)

// windowsBootstrapConfig is the Windows-only installer hook. If set, this
// will trigger hydrating %ProgramData% from the etc folder the installer laid
// down under %ProgramFiles%.
var windowsBootstrapConfig = flag.Bool("windows-bootstrap-config", false, "if true, seed and harden the Windows config directory, then exit (used by the MSI installer)")

// programData is the directory Windows keeps machine-wide application state
// in. It is C:\ProgramData on a stock install, but it is relocatable and
// enterprise images do relocate it, so nothing here may assume the C: path.
//
// It is empty when Windows did not tell us where it is. Callers must check,
// because filepath.Join would otherwise turn an unset ProgramData into the
// relative path "Techaro\Anubis", writing the signing key somewhere
// unpredictable and hardening a directory that is not the one in use.
var programData = os.Getenv("ProgramData")

// dataDir is where the MSI installs the live configuration, the policy file
// and the logs. It is empty exactly when programData is.
var dataDir = func() string {
	if programData == "" {
		return ""
	}

	return filepath.Join(programData, "Techaro", "Anubis")
}()

// bootstrapFiles are copied out of the installer's etc folder on first install.
var bootstrapFiles = []string{"anubis.env", "anubis.yaml"}

// bootstrapLogName is the file the installer's bootstrap run writes its
// diagnostics to. See writeBootstrapLog for where it ends up.
const bootstrapLogName = "anubis-bootstrap.log"

// platformStartup prepares a service process before flags are parsed.
//
// A Windows service starts with no usable stderr and with its working
// directory set to the system folder, so the godotenv autoload import finds
// nothing and anything written to stderr is discarded. Both are fixed here,
// before any code can log or read a flag.
func platformStartup() {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		return
	}

	if dataDir == "" {
		// Nothing to redirect to and no config file to find. Anubis will come
		// up on defaults and fail somewhere more legible than here.
		return
	}

	// XXX(Xe): overwrite os.Stderr with anubis-startup.log. This is done because
	// msiexec sucks. See the doc comment for handleBootstrapFlag.
	redirectStderr(filepath.Join(dataDir, "anubis-startup.log"))

	// Load, not Overload: a real environment variable set on the service must
	// win over the file, matching how the Linux packages behave.
	if err := godotenv.Load(filepath.Join(dataDir, "anubis.env")); err != nil {
		log.Printf("cannot load %s: %v", filepath.Join(dataDir, "anubis.env"), err)
	}
}

// redirectStderr points os.Stderr and the standard logger at path.
//
// Failures are silent because there is nowhere left to report them to.
func redirectStderr(path string) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o640)
	if err != nil {
		return
	}

	os.Stderr = f
	log.SetOutput(f)
}

// handleBootstrapFlag runs the installation configuration bootstrap when
// --windows-bootstrap-config is set.
//
// Due to facts and circumstances beyond my control, msiexec discards _all_
// logging messages when it runs Anubis with the --windows-bootstrap-config
// set during install. In order to have _some_ kind of debugging surface,
// we have to write logs to %ProgramData%\Techaro\Anubis\anubis-bootstrap.log.
//
// Additionally, things here have to return successful error codes even when
// operations fail because if this returns a non-success exit code then it
// surfaces as the obscure msiexec "Error 1603" without any details.
//
// I really hate this, but I don't really see a better option here.
func handleBootstrapFlag() bool {
	if !*windowsBootstrapConfig {
		return false
	}

	var buf bytes.Buffer
	lg := log.New(io.MultiWriter(os.Stderr, &buf), "", log.LstdFlags|log.LUTC)

	lg.Printf("bootstrapping the Anubis config directory")
	lg.Printf("ProgramData is %q, config directory is %q", programData, dataDir)

	err := bootstrapConfigDir(lg)
	if err != nil {
		lg.Printf("bootstrap failed: %v", err)
	} else {
		lg.Printf("bootstrap finished")
	}

	writeBootstrapLog(buf.Bytes())

	if err != nil {
		os.Exit(1)
	}

	return true
}

// bootstrapConfigDir seeds and hardens the config directory.
func bootstrapConfigDir(lg *log.Logger) error {
	if dataDir == "" {
		return errors.New("ProgramData is not set, refusing to guess where the configuration directory is")
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find my own path: %w", err)
	}

	// The installer lays the binary down in <prefix>\bin and the templates in
	// <prefix>\etc.
	srcDir := filepath.Join(filepath.Dir(filepath.Dir(exe)), "etc")

	lg.Printf("copying %v out of %q", bootstrapFiles, srcDir)

	return runBootstrap(bootstrapConfig{
		SrcDir:    srcDir,
		BaseDir:   programData,
		DestDir:   dataDir,
		Files:     bootstrapFiles,
		DataDir:   dataDir,
		Harden:    hardenDataDir,
		VerifyDir: verifyDataDir,
	})
}

// writeBootstrapLog appends the bootstrap's diagnostics to the first place
// it can be written to.
//
// Normally it writes to %ProgramData%\Techaro\Anubis\anubis-bootstrap.log,
// but if it can't then it just makes a temporary folder in C:\Windows\Temp
// and writes them there.
//
// Hopefully this fallback logic never runs, but sometimes you gotta have
// a way to fall back.
//
// Failures in this process are silent because there is nowhere left to
// report them to.
func writeBootstrapLog(body []byte) {
	var paths []string
	if dataDir != "" {
		paths = append(paths, filepath.Join(dataDir, bootstrapLogName))
	}
	paths = append(paths, filepath.Join(os.TempDir(), bootstrapLogName))

	for _, path := range paths {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o640)
		if err != nil {
			continue
		}

		_, writeErr := f.Write(body)
		closeErr := f.Close()

		if writeErr == nil && closeErr == nil {
			return
		}
	}
}

// hardenDataDir hardens the permissions of the Anubis data directory within an
// inch of its life.
//
// As mentioned before, %ProgramData%'s ACLs are a bit fucky out of the gate. In
// order to prevent a misconfigured system from resulting in normal users leaking
// Anubis secrets, the ACLs of the Anubis data directory are super-hardened such
// that only the Anubis service and Administrators can read them.
//
// Even more fun, usernames like "Administrator" are localized when Windows has
// its local set to non-English on install. As a result you need to use the raw
// identifiers instead of human-readable names.
//
// This is probably too paranoid and will need to be edited but it's better to
// start out more paranoid out of the gate.
func hardenDataDir(dir string) error {
	cmd := exec.Command(
		filepath.Join(os.Getenv("SystemRoot"), "System32", "icacls.exe"),
		dir,
		"/inheritance:r",
		"/grant", "*S-1-5-18:(OI)(CI)F",
		"/grant", "*S-1-5-32-544:(OI)(CI)F",
		"/T",
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("icacls failed: %w: %s", err, out)
	}

	return nil
}

// verifyDataDir makes sure that the directory in question is both a real
// directory and is owned by either LocalSystem or the local Administrators
// group.
//
// This is annoying to implement because of directory junctions and other
// magic NTFS features.
func verifyDataDir(dir string) error {
	name, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", dir, err)
	}

	// FILE_FLAG_BACKUP_SEMANTICS is what makes CreateFile willing to open a
	// directory at all. READ_CONTROL is the only access an owner read needs,
	// and asking for no more than that keeps this from failing over a
	// directory something else already has open.
	h, err := windows.CreateFile(
		name,
		windows.READ_CONTROL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", dir, err)
	}
	defer windows.CloseHandle(h)

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return fmt.Errorf("cannot read attributes of %s: %w", dir, err)
	}

	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("%w: %s is a reparse point", ErrUntrustedDir, dir)
	}

	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return fmt.Errorf("%w: %s is not a directory", ErrUntrustedDir, dir)
	}

	sd, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("cannot read owner of %s: %w", dir, err)
	}

	owner, _, err := sd.Owner()
	if err != nil {
		return fmt.Errorf("cannot read owner of %s: %w", dir, err)
	}

	localSystem, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return fmt.Errorf("cannot build LocalSystem SID: %w", err)
	}

	administrators, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return fmt.Errorf("cannot build Administrators SID: %w", err)
	}

	if owner.Equals(localSystem) || owner.Equals(administrators) {
		return nil
	}

	return fmt.Errorf("%w: %s is owned by %s, want LocalSystem (S-1-5-18) or Administrators (S-1-5-32-544)", ErrUntrustedDir, dir, owner)
}

// runPlatformService runs fn under the service control manager when this
// process was started as a Windows service. It reports whether it did so.
func runPlatformService(fn func(context.Context)) bool {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		return false
	}

	if err := svc.Run("Anubis", &anubisService{fn: fn}); err != nil {
		log.Fatalf("service failed: %v", err)
	}

	return true
}

// anubisService adapts run to the service control manager's interface.
type anubisService struct {
	fn func(context.Context)
}

// startPollInterval is how often Execute asks whether Anubis is serving yet.
const startPollInterval = 100 * time.Millisecond

// startWaitHint is how long the service control manager is told to expect
// between two checkpoints while the service is starting.
const startWaitHint = 30 * time.Second

// Execute implements svc.Handler. It starts Anubis in the background, waits
// for it to actually be serving before reporting the service as running, and
// translates a stop or shutdown request into cancellation of its context.
func (s *anubisService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.fn(ctx)
	}()

	if !waitUntilServing(r, changes, done) {
		// Anubis gave up before it ever served a request. Returning a
		// service-specific error makes "sc start anubis" fail and puts a 7024
		// in the event log, rather than the service reporting a clean start
		// and then disappearing for reasons nobody wrote down.
		//
		// svc.Run reports the final Stopped status itself, using exactly
		// these two return values, so sending one here would only report a
		// clean stop a moment before the real one.
		cancel()
		<-done

		return true, 1
	}

	changes <- svc.Status{State: svc.Running, Accepts: accepted}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			}
		case <-done:
			// Anubis stopped on its own, which the control manager treats as
			// the service exiting.
			return false, 0
		}
	}
}

// waitUntilServing blocks until Anubis is listening and known healthy.
//
// Nearly everything that can go wrong with starting Anubis will happen
// while the service is managed by the Windows service manager. Windows'
// service management subsystem will wait for the service to be marked
// as running before `sc start Anubis` or `Start-Service Anubis` return.
//
// This interrogates Anubis' health every 100ms until it starts
// successfully. In most cases this will iterate once.
func waitUntilServing(r <-chan svc.ChangeRequest, changes chan<- svc.Status, done <-chan struct{}) bool {
	status := svc.Status{
		State:    svc.StartPending,
		WaitHint: uint32(startWaitHint / time.Millisecond),
	}
	changes <- status

	tick := time.NewTicker(startPollInterval)
	defer tick.Stop()

	for {
		select {
		case c := <-r:
			// Stop is not in Accepts yet, so an interrogation is the only
			// thing that should arrive here.
			if c.Cmd == svc.Interrogate {
				changes <- status
			}
		case <-done:
			return false
		case <-tick.C:
			if st, ok := internal.GetHealth("anubis"); ok && st == healthv1.HealthCheckResponse_SERVING {
				return true
			}

			status.CheckPoint++
			changes <- status
		}
	}
}
