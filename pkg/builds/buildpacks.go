package builds

import (
	"sort"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetBuildPacks returns a map of the BuildPacks along with the correctly ordered names
func GetBuildPacks(jxClient versioned.Interface, ns string) (map[string]*v1.BuildPack, []string, error) {
	m := map[string]*v1.BuildPack{}

	names := []string{}
	list, err := jxClient.JenkinsV1().BuildPacks(ns).List(metav1.ListOptions{})
	if err != nil {
		return m, names, err
	}
	if len(list.Items) == 0 {
		list.Items = createDefaultBuildBacks()
	}
	SortBuildPacks(list.Items)
	for _, resource := range list.Items {
		n := resource.Spec.Label
		copy := resource
		m[n] = &copy
		if n != "" {
			names = append(names, n)
		}
	}
	return m, names, nil
}

// createDefaultBuildBacks creates the default build packs if there are no BuildPack CRDs registered in a cluster
func createDefaultBuildBacks() []v1.BuildPack {
	return []v1.BuildPack{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubernetes-workloads",
			},
			Spec: v1.BuildPackSpec{
				Label:  "Kubernetes Workloads: Automated CI+CD with GitOps Promotion",
				GitURL: v1.KubernetesWorkloadBuildPackURL,
				GitRef: v1.KubernetesWorkloadBuildPackRef,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "classic-workloads",
			},
			Spec: v1.BuildPackSpec{
				Label:  "Library Workloads: CI+Release but no CD",
				GitURL: v1.ClassicWorkloadBuildPackURL,
				GitRef: v1.ClassicWorkloadBuildPackRef,
			},
		},
	}
}

// BuildPackOrder used to sort the build packs in label order
type BuildPackOrder []v1.BuildPack

func (a BuildPackOrder) Len() int      { return len(a) }
func (a BuildPackOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BuildPackOrder) Less(i, j int) bool {
	b1 := a[i]
	b2 := a[j]
	o1 := b1.Spec.Label
	o2 := b2.Spec.Label
	if o1 == o2 {
		return b1.Name < b2.Name
	}
	return o1 < o2
}

// SortBuildPacks sorts the build packs in order
func SortBuildPacks(buildPacks []v1.BuildPack) {
	sort.Sort(BuildPackOrder(buildPacks))
}
