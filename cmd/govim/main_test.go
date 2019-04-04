// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kr/pty"
	"github.com/myitcv/govim/testdriver"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"vim":     testdriver.Vim,
		"execvim": execvim,
	}))
}

func TestScripts(t *testing.T) {
	var wg sync.WaitGroup
	errCh := make(chan error)

	goplspath := strings.TrimSpace(runCmd(t, "gobin", "-m", "-p", "golang.org/x/tools/cmd/gopls"))
	plugpath := strings.TrimSpace(runCmd(t, "go", "list", "-m", "-f={{.Dir}}"))

	t.Run("scripts", func(t *testing.T) {
		testscript.Run(t, testscript.Params{
			Dir: "testdata",
			Setup: func(e *testscript.Env) error {
				e.Vars = append(e.Vars,
					"PLUGIN_PATH="+plugpath,
					"CURRENT_GOPATH="+os.Getenv("GOPATH"),
				)
				wg.Add(1)
				d := newplugin(string(goplspath))
				td, err := testdriver.NewTestDriver(filepath.Base(e.WorkDir), e, errCh, d)
				if err != nil {
					t.Fatalf("failed to create new driver: %v", err)
				}
				if err := td.Run(); err != nil {
					td.Close()
					wg.Done()
					return err
				}
				e.Defer(func() {
					td.Close()
					wg.Done()
				})
				return nil
			},
		})
	})

	errsDone := make(chan bool)

	var errs []error

	go func() {
		for err, ok := <-errCh; ok; {
			errs = append(errs, err)
		}
		close(errsDone)
	}()

	go func() {
		wg.Wait()
		close(errCh)
	}()

	<-errsDone

	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		t.Fatalf("got some errors:\n%v\n", strings.Join(msgs, "\n"))
	}
}

func TestInstallScripts(t *testing.T) {
	if testing.Short() {
		t.Skip("Install scripts are long-running")
	}

	plugpath := strings.TrimSpace(runCmd(t, "go", "list", "-m", "-f={{.Dir}}"))

	t.Run("scripts", func(t *testing.T) {
		testscript.Run(t, testscript.Params{
			Dir: "testdatainstall",
			Setup: func(e *testscript.Env) error {
				e.Vars = append(e.Vars,
					"PLUGIN_PATH="+plugpath,
					"CURRENT_GOPATH="+os.Getenv("GOPATH"),
				)
				return nil
			},
		})
	})
}

func runCmd(t *testing.T, c string, args ...string) string {
	t.Helper()
	cmd := exec.Command(c, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run %v: %v\n%s", strings.Join(cmd.Args, " "), err, out)
	}
	return string(out)
}

func execvim() int {
	args := append([]string{"--not-a-term"}, os.Args[1:]...)
	cmd := exec.Command("vim", args[1:]...)
	thepty, err := pty.Start(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start %v: %v", strings.Join(cmd.Args, " "), err)
		return 1
	}
	go io.Copy(ioutil.Discard, thepty)
	if err := cmd.Wait(); err != nil {
		return 1
	}
	return 0
}
