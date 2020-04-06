package kube

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
)

type NodeStatus struct {
	Name               string
	AllocatableMemory  *resource.Quantity
	AllocatableCPU     *resource.Quantity
	AllocatableStorage *resource.Quantity

	CpuRequests resource.Quantity
	CpuLimits   resource.Quantity
	MemRequests resource.Quantity
	MemLimits   resource.Quantity

	DiskRequests              resource.Quantity
	DiskLimits                resource.Quantity
	numberOfNonTerminatedPods int
}

type ClusterStatus struct {
	Name                   string
	nodeCount              int
	totalUsedMemory        resource.Quantity
	totalUsedCpu           resource.Quantity
	totalAllocatableMemory resource.Quantity
	totalAllocatableCpu    resource.Quantity
}

func GetClusterStatus(client kubernetes.Interface, namespace string, verbose bool) (ClusterStatus, error) {

	clusterStatus := ClusterStatus{
		totalAllocatableCpu:    resource.Quantity{},
		totalAllocatableMemory: resource.Quantity{},
		totalUsedCpu:           resource.Quantity{},
		totalUsedMemory:        resource.Quantity{},
	}

	kuber := NewKubeConfig()
	config, _, err := kuber.LoadConfig()
	if err != nil {
		return clusterStatus, err
	}

	if config != nil {
		context := CurrentContext(config)
		if context != nil {
			clusterStatus.Name = context.Cluster
		}
	}

	nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{})

	if err != nil {
		return clusterStatus, err
	}
	clusterStatus.nodeCount = len(nodes.Items)
	if clusterStatus.nodeCount < 1 {
		msg := fmt.Sprintf("Number of nodes in cluster  = %d which is not sufficient", clusterStatus.nodeCount)
		log.Logger().Fatal(msg)
		err = errors.NewServiceUnavailable(msg)
		return clusterStatus, err
	}
	for _, node := range nodes.Items {
		if verbose {

			log.Logger().Infof("\n-------------------------")
			log.Logger().Infof("Node:\n%s\n", node.Name)
		}

		nodeStatus, err := Status(client, namespace, node, verbose)
		if err != nil {
			return clusterStatus, err
		}
		clusterStatus.totalAllocatableMemory.Add(*nodeStatus.AllocatableMemory)
		clusterStatus.totalAllocatableCpu.Add(*nodeStatus.AllocatableCPU)

		clusterStatus.totalUsedMemory.Add(nodeStatus.MemRequests)
		clusterStatus.totalUsedCpu.Add(nodeStatus.CpuRequests)
	}

	return clusterStatus, nil
}

func (clusterStatus *ClusterStatus) MinimumResourceLimit() int {
	return 80
}

func (clusterStatus *ClusterStatus) AverageCpuPercent() int {
	return int((clusterStatus.totalUsedCpu.Value() * 100) / clusterStatus.totalAllocatableCpu.Value())
}

func (clusterStatus *ClusterStatus) AverageMemPercent() int {
	return int((clusterStatus.totalUsedMemory.Value() * 100) / clusterStatus.totalAllocatableMemory.Value())
}

func (clusterStatus *ClusterStatus) NodeCount() int {
	return clusterStatus.nodeCount
}

func (clusterStatus *ClusterStatus) CheckResource() string {
	status := []string{}
	if clusterStatus.AverageMemPercent() >= clusterStatus.MinimumResourceLimit() {
		status = append(status, "needs more free memory")
	}
	if clusterStatus.AverageCpuPercent() >= clusterStatus.MinimumResourceLimit() {
		status = append(status, "needs more free compute")
	}
	return strings.Join(status, ", ")
}

func (clusterStatus *ClusterStatus) Info() string {
	str := fmt.Sprintf("Cluster(%s): %d nodes, memory %d%% of %s, cpu %d%% of %s",
		clusterStatus.Name,
		clusterStatus.NodeCount(),
		clusterStatus.AverageMemPercent(),
		clusterStatus.totalAllocatableMemory.String(),
		clusterStatus.AverageCpuPercent(),
		clusterStatus.totalAllocatableCpu.String())
	return str
}

func Status(client kubernetes.Interface, namespace string, node v1.Node, verbose bool) (NodeStatus, error) {
	nodeStatus := NodeStatus{}
	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + node.Name + ",status.phase!=" + string(v1.PodSucceeded) + ",status.phase!=" + string(v1.PodFailed))
	if err != nil {
		return nodeStatus, err
	}

	allocatable := node.Status.Capacity
	if len(node.Status.Allocatable) > 0 {
		allocatable = node.Status.Allocatable
	}

	nodeStatus.Name = node.Name

	nodeStatus.AllocatableCPU = allocatable.Cpu()
	nodeStatus.AllocatableMemory = allocatable.Memory()
	nodeStatus.AllocatableStorage = allocatable.StorageEphemeral()

	// in a policy aware setting, users may have access to a node, but not all pods
	// in that case, we note that the user does not have access to the pods

	nodeNonTerminatedPodsList, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{FieldSelector: fieldSelector.String()})
	if err != nil {
		if !errors.IsForbidden(err) {
			return nodeStatus, err
		}
	}

	nodeStatus.numberOfNonTerminatedPods = len(nodeNonTerminatedPodsList.Items)

	reqs, limits := getPodsTotalRequestsAndLimits(nodeNonTerminatedPodsList, verbose)

	cpuReqs, cpuLimits, memoryReqs, memoryLimits, diskReqs, diskLimits := reqs[v1.ResourceCPU], limits[v1.ResourceCPU], reqs[v1.ResourceMemory], limits[v1.ResourceMemory], reqs[v1.ResourceEphemeralStorage], limits[v1.ResourceEphemeralStorage]

	if verbose {
		cpuPercent := (cpuReqs.Value() * 100) / allocatable.Cpu().Value()
		cpuMessage := ""
		if cpuReqs.Value() > allocatable.Cpu().Value() {
			cpuMessage = " - Node appears to be overcommitted on CPU"
		}

		log.Logger().Infof("CPU usage %v%% %s", cpuPercent, util.ColorWarning(cpuMessage))

		memoryPercent := (memoryReqs.Value() * 100) / allocatable.Memory().Value()
		memoryMessage := ""
		if memoryReqs.Value() > allocatable.Memory().Value() {
			memoryMessage = " - Node appears to be overcommitted on Memory"
		}

		log.Logger().Infof("Memory usage %v%%%s", memoryPercent, util.ColorWarning(memoryMessage))
	}

	nodeStatus.CpuRequests = cpuReqs
	nodeStatus.CpuLimits = cpuLimits
	nodeStatus.MemRequests = memoryReqs
	nodeStatus.MemLimits = memoryLimits

	nodeStatus.DiskRequests = diskReqs
	nodeStatus.DiskLimits = diskLimits

	return nodeStatus, nil
}

func getPodsTotalRequestsAndLimits(podList *v1.PodList, verbose bool) (reqs map[v1.ResourceName]resource.Quantity, limits map[v1.ResourceName]resource.Quantity) {
	reqs, limits = map[v1.ResourceName]resource.Quantity{}, map[v1.ResourceName]resource.Quantity{}

	if verbose {
		log.Logger().Infof("Pods:")
	}

	for _, pod := range podList.Items {
		podReqs, podLimits := PodRequestsAndLimits(&pod)

		if verbose {
			messages := []string{}

			if _, ok := podReqs[v1.ResourceCPU]; !ok {
				messages = append(messages, "No CPU request set")
			}

			if _, ok := podReqs[v1.ResourceMemory]; !ok {
				messages = append(messages, "No Memory request set")
			}

			log.Logger().Infof("%s - %s %s", pod.Name, pod.Status.Phase, util.ColorError(strings.Join(messages, ", ")))
		}

		for podReqName, podReqValue := range podReqs {
			if value, ok := reqs[podReqName]; !ok {
				reqs[podReqName] = *podReqValue.Copy()
			} else {
				value.Add(podReqValue)
				reqs[podReqName] = value
			}
		}
		for podLimitName, podLimitValue := range podLimits {
			if value, ok := limits[podLimitName]; !ok {
				limits[podLimitName] = *podLimitValue.Copy()
			} else {
				value.Add(podLimitValue)
				limits[podLimitName] = value
			}
		}
	}

	if verbose {
		log.Logger().Infof("")
	}

	return
}

func PodRequestsAndLimits(pod *v1.Pod) (reqs map[v1.ResourceName]resource.Quantity, limits map[v1.ResourceName]resource.Quantity) {
	reqs, limits = map[v1.ResourceName]resource.Quantity{}, map[v1.ResourceName]resource.Quantity{}
	for _, container := range pod.Spec.Containers {
		for name, quantity := range container.Resources.Requests {
			if value, ok := reqs[name]; !ok {
				reqs[name] = *quantity.Copy()
			} else {
				value.Add(quantity)
				reqs[name] = value
			}
		}
		for name, quantity := range container.Resources.Limits {
			if value, ok := limits[name]; !ok {
				limits[name] = *quantity.Copy()
			} else {
				value.Add(quantity)
				limits[name] = value
			}
		}
	}
	// init containers define the minimum of any resource
	for _, container := range pod.Spec.InitContainers {
		for name, quantity := range container.Resources.Requests {
			value, ok := reqs[name]
			if !ok {
				reqs[name] = *quantity.Copy()
				continue
			}
			if quantity.Cmp(value) > 0 {
				reqs[name] = *quantity.Copy()
			}
		}
		for name, quantity := range container.Resources.Limits {
			value, ok := limits[name]
			if !ok {
				limits[name] = *quantity.Copy()
				continue
			}
			if quantity.Cmp(value) > 0 {
				limits[name] = *quantity.Copy()
			}
		}
	}
	return
}

func RoleBindings(client kubernetes.Interface, namespace string) (string, error) {
	binding, err := client.Rbac().RoleBindings(namespace).Get("", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	result := ""

	for _, s := range binding.Subjects {
		result += fmt.Sprintf("%s\t%s\t%s\n", s.Kind, s.Name, s.Namespace)
	}

	return result, nil
}
