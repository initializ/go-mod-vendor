package gomodvendor_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	gomodvendor "github.com/initializ/go-mod-vendor"
	"github.com/paketo-buildpacks/go-mod-vendor/fakes"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/pexec"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testModVendor(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		workingDir  string
		environment []string
		executable  *fakes.Executable
		logs        *bytes.Buffer

		modVendor gomodvendor.ModVendor
	)

	it.Before(func() {
		var err error
		workingDir, err = os.MkdirTemp("", "working-directory")
		Expect(err).NotTo(HaveOccurred())

		environment = os.Environ()
		executable = &fakes.Executable{}

		logs = bytes.NewBuffer(nil)

		now := time.Now()
		times := []time.Time{now, now.Add(1 * time.Second)}

		clock := chronos.NewClock(func() time.Time {
			if len(times) == 0 {
				return time.Now()
			}

			t := times[0]
			times = times[1:]
			return t
		})

		modVendor = gomodvendor.NewModVendor(executable, scribe.NewEmitter(logs), clock)
	})

	it.After(func() {
		Expect(os.RemoveAll(workingDir)).To(Succeed())
	})

	context("ShouldRun", func() {
		context("when there is a vendor directory present", func() {
			it.Before(func() {
				Expect(os.Mkdir(filepath.Join(workingDir, "vendor"), os.ModePerm)).To(Succeed())
			})

			it("returns false", func() {
				ok, reason, err := modVendor.ShouldRun(workingDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeFalse())
				Expect(reason).To(Equal("modules are already vendored"))
			})
		})

		context("failure cases", func() {
			context("the vendor dir stat fails", func() {
				it.Before(func() {
					Expect(os.Chmod(workingDir, 0000)).To(Succeed())
				})

				it.After(func() {
					Expect(os.Chmod(workingDir, os.ModePerm)).To(Succeed())
				})

				it("returns an error", func() {
					_, _, err := modVendor.ShouldRun(workingDir)
					Expect(err).To(MatchError(ContainSubstring("permission denied")))
				})
			})
		})
	})

	context("Execute", func() {
		it.Before(func() {
			executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
				fmt.Fprintln(execution.Stdout, "stdout-output")
				fmt.Fprintln(execution.Stderr, "stderr-output")
				return nil
			}
		})
		it("runs go mod vendor", func() {
			err := modVendor.Execute("mod-cache-path", workingDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(executable.ExecuteCall.Receives.Execution.Args).To(Equal([]string{"mod", "vendor"}))
			Expect(executable.ExecuteCall.Receives.Execution.Env).To(Equal(append(environment, fmt.Sprintf("GOMODCACHE=%s", "mod-cache-path"))))
			Expect(executable.ExecuteCall.Receives.Execution.Dir).To(Equal(workingDir))

			Expect(logs.String()).To(ContainSubstring("  Executing build process"))
			Expect(logs.String()).To(ContainSubstring("    Running 'go mod vendor'"))
			Expect(logs.String()).To(ContainSubstring("      stdout-output"))
			Expect(logs.String()).To(ContainSubstring("      stderr-output"))
			Expect(logs.String()).To(ContainSubstring("      Completed in 1s"))
		})

		context("failure cases", func() {
			context("the executable fails", func() {
				it.Before(func() {
					executable.ExecuteCall.Stub = func(execution pexec.Execution) error {
						fmt.Fprintln(execution.Stdout, "build error stdout")
						fmt.Fprintln(execution.Stderr, "build error stderr")

						return errors.New("executable failed")
					}
				})

				it("returns an error", func() {
					err := modVendor.Execute("mod-cache-path", workingDir)
					Expect(err).To(MatchError(ContainSubstring("executable failed")))

					Expect(logs.String()).To(ContainSubstring("      build error stdout"))
					Expect(logs.String()).To(ContainSubstring("      build error stderr"))
					Expect(logs.String()).To(ContainSubstring("      Failed after 1s"))
				})
			})
		})
	})
}
