package test

import (
	_ "embed"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"html/template"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

//go:embed testdata/haproxy.cfg.templ
var haproxyConfigTemplateContents string

//go:embed testdata/anubis.cfg
var haproxyAgentConfig string

//go:embed testdata/ok.http
var okResponse string

var haproxyConfigTemplate = func() *template.Template {
	template, err := template.New("haproxyConfig").Parse(haproxyConfigTemplateContents)
	if err != nil {
		panic(err)
	}
	return template
}()

func TestWithRealHAProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping haproxy tests in short mode")
		return
	}

	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("SKIP_INTEGRATION was set")
		return
	}

	if _, err := exec.LookPath("haproxy"); err != nil {
		t.Skip("haproxy not found in path")
		return
	}

	connInfo := spawnAnubisWithOptions(t, anubisTestOptions{enableSPOE: true})

	listenURL := spawnHaproxy(t, connInfo)

	client := http.Client{}

	r, err := client.Get(listenURL)
	if err != nil {
		t.Fatal(err)
	}

	got, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(string(got), "OK"); diff != "" {
		t.Fatalf("HTTP response mismatch (-want +got):\n%s", diff)
	}
}

func spawnHaproxy(t *testing.T, anubis anubisConnectionInfo) string {
	workdir := t.TempDir()

	configPath := filepath.Join(workdir, "haproxy.cfg")
	agentConfigPath := filepath.Join(workdir, "anubis.cfg")
	okFile := filepath.Join(workdir, "ok.http")

	// We need to pick a port for HAProxy, as we do not have the option to figure out which port it has bound to
	haproxyPort := uint16((rand.Uint32() & 0xffff) | 0xc000)

	f, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	anubisUrl, err := url.Parse(anubis.URL)
	if err != nil {
		t.Fatal(err)
	}
	anubisPort, err := strconv.ParseUint(anubisUrl.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}

	templateData := struct {
		HAProxyBindPort uint16
		SPOAConfig      string
		AnubisPort      uint16
		AnubisSpoePort  uint16
		OKResponse      string
	}{
		HAProxyBindPort: haproxyPort,
		SPOAConfig:      agentConfigPath,
		AnubisPort:      uint16(anubisPort),
		AnubisSpoePort:  anubis.spoePort,
		OKResponse:      okFile,
	}

	if err := haproxyConfigTemplate.Execute(f, templateData); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(agentConfigPath, []byte(haproxyAgentConfig), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(okFile, []byte(okResponse), 0600); err != nil {
		t.Fatal(err)
	}

	haproxyCommand := exec.Command("haproxy", "-f", configPath)
	haproxyCommand.Stdout = t.Output()
	haproxyCommand.Stderr = t.Output()

	if err := haproxyCommand.Start(); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		t.Helper()
		t.Log("killing haproxy process")
		haproxyCommand.Process.Kill()
	})

	// We wait until the bound socket is reachable, for a maximum of ten seconds. HAProxy takes a bit of time to start up
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)

		s, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", haproxyPort))
		if err == nil {
			s.Close()
			t.Log("HAProxy ready to serve requests")
			return fmt.Sprintf("http://localhost:%d", haproxyPort)
		}
	}
	t.Fatal("HAProxy startup timed out")
	return ""
}
