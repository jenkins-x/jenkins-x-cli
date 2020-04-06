package opts

import (
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
)

// TailLogs returns the logs from a given pod
func (o *CommonOptions) TailLogs(ns string, pod string, containerName string) error {
	return errors.WithStack(kube.TailLogs(ns, pod, containerName, o.Err, o.Out))
}
