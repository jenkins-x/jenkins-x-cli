package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const (
	kubeHunterImage         = "aquasec/kube-hunter"
	kubeHunterContainerName = "jx-kube-hunter"
	kubeHunterNamespace     = "jx-kube-hunter"
	kubeHunterJobName       = "jx-kube-hunter-job"

	outputFormatYAML = "yaml"
)

// ScanClusterOptions the options for 'scan cluster' command
type ScanClusterOptions struct {
	ScanOptions

	Output string
}

type node struct {
	Type     string `json:"type" yaml:"type"`
	Location string `json:"location" yaml:"location"`
}

type service struct {
	Service     string `json:"service" yaml:"service"`
	Location    string `json:"location" yaml:"location"`
	Description string `json:"description" yaml:"description"`
}

type vulnerability struct {
	Vulnerability string `json:"vulnerability" yaml:"vulnerability"`
	Location      string `json:"location" yaml:"location"`
	Category      string `json:"category" yaml:"category"`
	Description   string `json:"description" yaml:"description"`
	Evidence      string `json:"evidence" yaml:"evidence"`
}

type scanResult struct {
	Nodes           []node          `json:"nodes" yaml:"nodes"`
	Services        []service       `json:"services" yaml:"services"`
	Vulnerabilities []vulnerability `json:"vulnerabilities" yaml:"vulnerabilities"`
}

// NewCmdScanCluster creates a command object for "scan cluster" command
func NewCmdScanCluster(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &ScanClusterOptions{
		ScanOptions: ScanOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Performs a cluster security scan",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Output, "output", "o", "plain", "output format is one of: yaml|plain")

	return cmd
}

// Run executes the "scan cluster" command
func (o *ScanClusterOptions) Run() error {
	kubeClient, err := o.KubeClient()
	if err != nil {
		return errors.Wrap(err, "creating kube client")
	}

	// Create a dedicated namespace for kube-hunter scan
	ns := kubeHunterNamespace
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	_, err = kubeClient.CoreV1().Namespaces().Create(namespace)
	if err != nil {
		return errors.Wrapf(err, "creating namespace '%s'", ns)
	}

	// Start the kube-hunter scanning
	container := o.hunterContainer()
	job := o.createScanJob(kubeHunterJobName, ns, container)
	job, err = kubeClient.BatchV1().Jobs(ns).Create(job)
	if err != nil {
		return err
	}

	// Wait for scanning to complete successfully
	err = kube.WaitForJobToSucceeded(kubeClient, ns, kubeHunterJobName, 3*time.Minute)
	if err != nil {
		return errors.Wrap(err, "waiting for kube hunter job to complete the scanning")
	}

	result, err := o.retriveScanResult(ns)
	if err != nil {
		return errors.Wrap(err, "retrieving scan result")
	}

	// Clean up the kube-hunter namespace
	err = kubeClient.CoreV1().Namespaces().Delete(ns, &metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "cleaning up the scanning namespace '%s'", ns)
	}

	scanResult, err := o.parseResult(result)
	if err != nil {
		return errors.Wrap(err, "parsing the scan result")
	}

	err = o.printResult(scanResult)
	if err != nil {
		return errors.Wrap(err, "printing the result")
	}

	// Signal the error in the exit code if there are any vulnerabilities
	foundVulns := len(scanResult.Vulnerabilities)
	if foundVulns > 0 {
		os.Exit(2)
	}

	return nil
}

func (o *ScanClusterOptions) hunterContainer() *v1.Container {
	return &v1.Container{
		Name:            kubeHunterContainerName,
		Image:           o.hunterImage(),
		ImagePullPolicy: v1.PullAlways,
		Command:         []string{"python", "kube-hunter.py"},
		Args:            []string{"--pod", "--report=yaml", "--log=none"},
	}
}

func (o *ScanClusterOptions) hunterImage() string {
	defaultImage := kubeHunterImage + ":latest"
	resolver, err := o.CreateVersionResolver("", "")
	if err != nil {
		return defaultImage
	}
	versionedImage, err := resolver.ResolveDockerImage(kubeHunterImage)
	if err != nil {
		return defaultImage
	}
	return versionedImage
}

func (o *ScanClusterOptions) createScanJob(name string, namespace string, container *v1.Container) *batchv1.Job {
	podTmpl := v1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers:    []v1.Container{*container},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: podTmpl,
		},
	}
}

func (o *ScanClusterOptions) retriveScanResult(namespace string) (string, error) {
	kubeClient, err := o.KubeClient()
	if err != nil {
		return "", errors.Wrap(err, "creating kube client")
	}

	labels := map[string]string{"job-name": kubeHunterJobName}
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: labels})
	podList, err := kubeClient.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return "", errors.Wrap(err, "listing the scan job PODs")
	}
	foundPods := len(podList.Items)
	if foundPods != 1 {
		return "", fmt.Errorf("one POD expected for security scan job '%s'. Found: %d PODs.", kubeHunterJobName, foundPods)
	}

	podName := podList.Items[0].Name
	logOpts := &v1.PodLogOptions{
		Container: kubeHunterContainerName,
		Follow:    false,
	}
	req := kubeClient.CoreV1().Pods(namespace).GetLogs(podName, logOpts)
	readCloser, err := req.Stream()
	if err != nil {
		return "", errors.Wrap(err, "creating the logs stream reader")
	}
	defer readCloser.Close()

	var result []byte
	reader := bufio.NewReader(readCloser)
	for {
		line, _, err := reader.ReadLine()
		if err != nil && err != io.EOF {
			return "", errors.Wrapf(err, "reading logs from POD  '%s'", podName)
		}
		if err == io.EOF {
			break
		}
		line = append(line, '\n')
		result = append(result, line...)
	}

	return string(result), nil
}

func (o *ScanClusterOptions) parseResult(result string) (*scanResult, error) {
	r := scanResult{}
	err := yaml.Unmarshal([]byte(result), &r)
	if err != nil {
		return nil, errors.Wrap(err, "parsing the YAML result")
	}
	return &r, nil
}

func (o *ScanClusterOptions) printResult(result *scanResult) error {
	if o.Output == outputFormatYAML {
		var output []byte
		output, err := yaml.Marshal(result)
		if err != nil {
			return errors.Wrap(err, "converting scan result to YAML")
		}
		log.Logger().Info(string(output))
	} else {
		nodeTable := o.CreateTable()
		nodeTable.SetColumnAlign(1, util.ALIGN_LEFT)
		nodeTable.SetColumnAlign(2, util.ALIGN_LEFT)
		nodeTable.AddRow("NODE", "LOCATION")
		for _, n := range result.Nodes {
			nodeTable.AddRow(n.Type, n.Location)
		}
		nodeTable.Render()
		log.Blank()

		serviceTable := o.CreateTable()
		serviceTable.SetColumnAlign(1, util.ALIGN_LEFT)
		serviceTable.SetColumnAlign(2, util.ALIGN_LEFT)
		serviceTable.SetColumnAlign(3, util.ALIGN_LEFT)
		serviceTable.AddRow("SERVICE", "LOCATION", "DESCRIPTION")
		for _, s := range result.Services {
			serviceTable.AddRow(s.Service, s.Location, s.Description)
		}
		serviceTable.Render()
		log.Blank()

		vulnTable := o.CreateTable()
		vulnTable.SetColumnAlign(1, util.ALIGN_LEFT)
		vulnTable.SetColumnAlign(2, util.ALIGN_LEFT)
		vulnTable.SetColumnAlign(3, util.ALIGN_LEFT)
		vulnTable.SetColumnAlign(4, util.ALIGN_LEFT)
		vulnTable.SetColumnAlign(5, util.ALIGN_LEFT)
		vulnTable.AddRow("VULNERABILITY", "LOCATION", "CATEGORY", "DESCRIPTION", "EVIDENCE")
		for _, vuln := range result.Vulnerabilities {
			vulnTable.AddRow(vuln.Vulnerability, vuln.Location, vuln.Category, vuln.Description, vuln.Evidence)
		}
		vulnTable.Render()
		log.Blank()
	}
	return nil
}
