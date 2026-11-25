package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/dockfin/dockfin/internal/version"
)

func TestPrintUsageAndVersion(t *testing.T) {
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	printUsage()
	_ = w.Close()
	os.Stderr = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{"dockfin migrate", "dockfin serve", "dockfin worker", "dockfin version"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage missing %s\n%s", want, text)
		}
	}

	oldArgs := os.Args
	oldOut := os.Stdout
	os.Args = []string{"dockfin", "version"}
	t.Cleanup(func() {
		os.Args = oldArgs
		os.Stdout = oldOut
	})
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = pw
	main()
	_ = pw.Close()
	os.Stdout = oldOut
	ver, err := io.ReadAll(pr)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(ver)) != "dockfin "+version.Version {
		t.Fatalf("version output %q", ver)
	}
}

func TestCLIRejectsUnknown(t *testing.T) {
	assertCLIExit(t, "DOCKFIN_CLI_CHILD", "unknown", []string{"dockfin", "nope"}, 1, "Usage:")
}

func TestCLINoArgsExits(t *testing.T) {
	assertCLIExit(t, "DOCKFIN_CLI_CHILD", "noargs", []string{"dockfin"}, 1, "dockfin serve")
}

func TestCLIMigrateRequiresConfig(t *testing.T) {
	t.Setenv("DOCKFIN_MASTER_KEY", "short")
	t.Setenv("DOCKFIN_PUBLIC_URL", "http://127.0.0.1:8000")
	t.Setenv("DOCKFIN_ENV", "development")
	if err := runMigrate(); err == nil || !strings.Contains(err.Error(), "MASTER_KEY") {
		t.Fatalf("%v", err)
	}
}

func assertCLIExit(t *testing.T, envKey, envVal string, args []string, code int, want string) {
	t.Helper()
	if os.Getenv(envKey) == envVal {
		os.Args = args
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
	cmd.Env = append(os.Environ(), envKey+"="+envVal)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != code {
		t.Fatalf("exit %v output %s", err, buf.String())
	}
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("output missing %q\n%s", want, buf.String())
	}
}
