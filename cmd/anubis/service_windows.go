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
	"path/filepath"
	"time"

	"github.com/TecharoHQ/anubis/internal"
	"github.com/TecharoHQ/anubis/internal/servicesid"
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

// bootstrapConfigDir seeds the config directory from the installer's templates.
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

	if err := runBootstrap(bootstrapConfig{
		SrcDir:  srcDir,
		DestDir: dataDir,
		Files:   bootstrapFiles,
		DataDir: dataDir,
	}); err != nil {
		return err
	}

	lg.Printf("granting %s (%s) access to %q", servicesid.AnubisServiceName, servicesid.AnubisServiceSID, dataDir)

	return grantServiceAccess(dataDir)
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

// grantServiceAccess gives the Anubis service read and write access to its own
// data directory.
//
// The directory otherwise keeps whatever it inherits from %ProgramData%, which
// grants SYSTEM and the administrators full control and says nothing at all
// about NT SERVICE\Anubis. Without this the service cannot read anubis.env or
// create anubis.log, so it dies on startup with a permission error.
//
// This deliberately does not shell out to icacls. icacls reverse-maps every SID
// it is handed back to an account name, and LSA will not map an NT SERVICE SID
// for a service that is not registered yet. The bootstrap runs before
// InstallServices, so icacls fails the whole invocation with error 1332 and
// applies none of it. The API below takes the SID as bytes and never asks LSA
// anything, which is what lets the grant happen ahead of the service.
//
// The ACE is inheritable, and SetNamedSecurityInfo pushes inheritable ACEs down
// to existing children, so files left behind by an older install pick it up too.
func grantServiceAccess(dir string) error {
	sid, err := windows.StringToSid(servicesid.AnubisServiceSID)
	if err != nil {
		return fmt.Errorf("cannot parse the %s service SID %s: %w", servicesid.AnubisServiceName, servicesid.AnubisServiceSID, err)
	}

	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("cannot read the permissions of %s: %w", dir, err)
	}

	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("cannot read the permissions of %s: %w", dir, err)
	}

	// Modify, which is what Windows calls this combination: enough to read the
	// config, write and rotate the logs, and delete the rotated ones. It leaves
	// out WRITE_DAC and WRITE_OWNER, so the service cannot widen its own grant.
	const modify = windows.FILE_GENERIC_READ |
		windows.FILE_GENERIC_WRITE |
		windows.FILE_GENERIC_EXECUTE |
		windows.DELETE

	merged, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: modify,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}}, dacl)
	if err != nil {
		return fmt.Errorf("cannot add %s to the permissions of %s: %w", servicesid.AnubisServiceSID, dir, err)
	}

	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, merged, nil); err != nil {
		return fmt.Errorf("cannot write the permissions of %s: %w", dir, err)
	}

	return nil
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
