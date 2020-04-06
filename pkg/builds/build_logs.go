package builds

import (
	"bufio"
	"bytes"
	"io"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// GetBuildLogsForPod returns the pod log for a Knative Build style build pod which is based on init containers
func GetBuildLogsForPod(podInterface v1.PodInterface, pod *corev1.Pod) ([]byte, error) {
	var buffer bytes.Buffer
	podName := pod.Name

	containers, _, _ := kube.GetContainersWithStatusAndIsInit(pod)

	for _, container := range containers {
		buffer.WriteString("Step: ")
		buffer.WriteString(container.Name)
		buffer.WriteString(":\n\n")

		logOpts := &corev1.PodLogOptions{
			Container: container.Name,
			Follow:    false,
		}
		req := podInterface.GetLogs(podName, logOpts)
		readCloser, err := req.Stream()
		if err != nil {
			return nil, errors.Wrap(err, "creating the logs stream reader")
		}
		defer readCloser.Close()

		reader := bufio.NewReader(readCloser)
		for {
			line, _, err := reader.ReadLine()
			if err != nil && err != io.EOF {
				return nil, errors.Wrapf(err, "reading logs from POD  '%s'", podName)
			}
			if err == io.EOF {
				break
			}
			buffer.WriteString(string(line))
			buffer.WriteString("\n")
		}
	}
	return buffer.Bytes(), nil
}
