package kube

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

// TailLogs will tail the logs for the pod in ns with containerName,
// returning when the logs are complete. It writes to errOut and out.
func TailLogs(ns string, pod string, containerName string, errOut io.Writer, out io.Writer) error {
	args := []string{"logs", "-n", ns, "-f"}
	if containerName != "" {
		args = append(args, "-c", containerName)
	}
	args = append(args, pod)
	name := "kubectl"
	e := exec.Command(name, args...)
	e.Stderr = errOut
	stdout, _ := e.StdoutPipe()

	os.Setenv("PATH", util.PathWithBinary())
	err := e.Start()
	if err != nil {
		log.Logger().Errorf("Error: Command failed  %s %s", name, strings.Join(args, " "))
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		m := scanner.Text()
		fmt.Fprintln(out, m)
		if m == "Finished: FAILURE" {
			os.Exit(1)
		}
	}
	e.Wait()
	return nil
}
