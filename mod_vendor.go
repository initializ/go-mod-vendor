package gomodvendor

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/cloudfoundry/packit/pexec"
	"github.com/paketo-buildpacks/packit/chronos"
)

//go:generate faux --interface Executable --output fakes/executable.go
type Executable interface {
	Execute(pexec.Execution) error
}

type ModVendor struct {
	executable Executable
	logs       LogEmitter
	clock      chronos.Clock
}

func NewModVendor(executable Executable, logs LogEmitter, clock chronos.Clock) ModVendor {
	return ModVendor{
		executable: executable,
		logs:       logs,
		clock:      clock,
	}
}

func (m ModVendor) Execute(path, workingDir string) error {

	m.logs.Process("Executing build process")
	m.logs.Subprocess("Running 'go mod vendor'")

	buffer := bytes.NewBuffer(nil)

	duration, err := m.clock.Measure(func() error {
		return m.executable.Execute(pexec.Execution{
			Args:   []string{"mod", "vendor"},
			Env:    append(os.Environ(), fmt.Sprintf("GOPATH=%s", path)),
			Dir:    workingDir,
			Stdout: buffer,
			Stderr: buffer,
		})
	})
	if err != nil {
		m.logs.Action("Failed after %s", duration.Round(time.Millisecond))
		m.logs.Detail(buffer.String())

		return err
	}

	m.logs.Action("Completed in %s", duration.Round(time.Millisecond))
	m.logs.Break()

	return nil
}