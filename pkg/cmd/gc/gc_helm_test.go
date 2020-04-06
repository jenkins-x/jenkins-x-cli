// +build unit

package gc_test

import (
	"bytes"
	"sort"
	"testing"
	"text/template"

	"github.com/jenkins-x/jx/pkg/cmd/gc"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/stretchr/testify/assert"
)

const (
	cmtemplate = `{
    "apiVersion": "v1",
    "data": {
        "release": "TESTDATA"
    },
    "kind": "ConfigMap",
    "metadata": {
        "creationTimestamp": "2018-04-26T22:56:12Z",
        "labels": {
            "MODIFIED_AT": "1525125621",
            "NAME": "{{.Name}}",
            "OWNER": "TILLER",
            "STATUS": "SUPERSEDED",
            "VERSION": "{{.Version}}"
        },
        "name": "{{.Name}}.v{{.Version}}",
        "namespace": "kube-system",
        "resourceVersion": "3983355",
        "selfLink": "/api/v1/namespaces/kube-system/configmaps/{{.Name}}.v{{.Version}}",
        "uid": "0447f9e9-49a5-11e8-95cd-42010a9a0000"
    }
}`
	cmlisttemplate = `{
    "apiVersion": "v1",
    "items": [{{.List}}],
    "kind": "ConfigMapList",
    "metadata": {
        "resourceVersion": "",
        "selfLink": ""
    }
}
`
	v_jx_staging    = 27
	v_jenkins_x     = 20
	v_jx_production = 3
)

type CMConfig struct {
	Name    string
	Version int
}

type CMListConfig struct {
	List string
}

func TestGCHelmSortVersion(t *testing.T) {
	t.Parallel()
	test_versions := []string{"jx-production.v2", "jx-production.v3", "jx-production.v1"}
	sort.Sort(gc.ByVersion(test_versions))
	assert.Equal(t, "jx-production.v1", test_versions[0])
	assert.Equal(t, "jx-production.v2", test_versions[1])
	assert.Equal(t, "jx-production.v3", test_versions[2])
}

func TestGCHelmSortVersionComplex(t *testing.T) {
	t.Parallel()
	test_versions := []string{"jx-p.v3.complex.v2", "jx-p.v1.complex.v3", "jx-p.v2.complex.v1"}
	sort.Sort(gc.ByVersion(test_versions))
	assert.Equal(t, "jx-p.v2.complex.v1", test_versions[0])
	assert.Equal(t, "jx-p.v3.complex.v2", test_versions[1])
	assert.Equal(t, "jx-p.v1.complex.v3", test_versions[2])

}

func TestGCHelmSortVersionMissing(t *testing.T) {
	t.Parallel()
	test_versions := []string{"aptly-broken3", "aptly-broken2", "aptly-broken1"}
	sort.Sort(gc.ByVersion(test_versions))
	assert.Equal(t, "aptly-broken3", test_versions[0])
	assert.Equal(t, "aptly-broken2", test_versions[1])
	assert.Equal(t, "aptly-broken1", test_versions[2])
}

func TestGCHelmExtract(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	b.WriteString(createConfigMaps(t, "jx-staging", v_jx_staging))
	b.WriteString(",")
	b.WriteString(createConfigMaps(t, "jenkins-x", v_jenkins_x))
	b.WriteString(",")
	b.WriteString(createConfigMaps(t, "jx-production", v_jx_production))
	configmaplist := createConfigMapList(t, b.String())

	releases := gc.ExtractReleases(configmaplist)

	assert.Contains(t, releases, "jx-staging")
	assert.Contains(t, releases, "jx-production")
	assert.Contains(t, releases, "jenkins-x")

	versions := gc.ExtractVersions(configmaplist, "jx-production")
	expected_versions := []string{"jx-production.v1", "jx-production.v2", "jx-production.v3"}
	assert.Equal(t, expected_versions, versions)

	to_delete := gc.VersionsToDelete(versions, 10)
	assert.Empty(t, to_delete)

	versions = gc.ExtractVersions(configmaplist, "jx-staging")
	expected_versions = []string{"jx-staging.v1", "jx-staging.v2", "jx-staging.v3", "jx-staging.v4", "jx-staging.v5", "jx-staging.v6", "jx-staging.v7", "jx-staging.v8", "jx-staging.v9", "jx-staging.v10", "jx-staging.v11", "jx-staging.v12", "jx-staging.v13", "jx-staging.v14", "jx-staging.v15", "jx-staging.v16", "jx-staging.v17", "jx-staging.v18", "jx-staging.v19", "jx-staging.v20", "jx-staging.v21", "jx-staging.v22", "jx-staging.v23", "jx-staging.v24", "jx-staging.v25", "jx-staging.v26", "jx-staging.v27"}
	assert.Equal(t, expected_versions, versions)

	to_delete = gc.VersionsToDelete(versions, 10)
	expected_to_delete := []string{"jx-staging.v1", "jx-staging.v2", "jx-staging.v3", "jx-staging.v4", "jx-staging.v5", "jx-staging.v6", "jx-staging.v7", "jx-staging.v8", "jx-staging.v9", "jx-staging.v10", "jx-staging.v11", "jx-staging.v12", "jx-staging.v13", "jx-staging.v14", "jx-staging.v15", "jx-staging.v16", "jx-staging.v17"}
	assert.Equal(t, expected_to_delete, to_delete)

	versions = gc.ExtractVersions(configmaplist, "jenkins-x")
	expected_versions = []string{"jenkins-x.v1", "jenkins-x.v2", "jenkins-x.v3", "jenkins-x.v4", "jenkins-x.v5", "jenkins-x.v6", "jenkins-x.v7", "jenkins-x.v8", "jenkins-x.v9", "jenkins-x.v10", "jenkins-x.v11", "jenkins-x.v12", "jenkins-x.v13", "jenkins-x.v14", "jenkins-x.v15", "jenkins-x.v16", "jenkins-x.v17", "jenkins-x.v18", "jenkins-x.v19", "jenkins-x.v20"}
	assert.Equal(t, expected_versions, versions)

	to_delete = gc.VersionsToDelete(versions, 10)
	expected_to_delete = []string{"jenkins-x.v1", "jenkins-x.v2", "jenkins-x.v3", "jenkins-x.v4", "jenkins-x.v5", "jenkins-x.v6", "jenkins-x.v7", "jenkins-x.v8", "jenkins-x.v9", "jenkins-x.v10"}
	assert.Equal(t, expected_to_delete, to_delete)

	versions = gc.ExtractVersions(configmaplist, "flaming-flamingo")
	assert.Empty(t, versions)

	cm, err := gc.ExtractConfigMap(configmaplist, "flaming-flamingo.v1")
	assert.NotNil(t, err)

	cm, err = gc.ExtractConfigMap(configmaplist, "jenkins-x.v1")
	assert.Nil(t, err)
	assert.NotNil(t, cm)

}

func createConfigMaps(t *testing.T, name string, versions int) string {
	var b bytes.Buffer
	cmtmpl := template.New("configmap")
	cmtmpl, err := cmtmpl.Parse(cmtemplate)
	assert.Nil(t, err)
	for i := 1; i <= versions; i++ {
		cmc := CMConfig{name, i}
		err1 := cmtmpl.Execute(&b, cmc)
		assert.Nil(t, err1)
		if i < versions {
			b.WriteString(",")
		}
	}
	return b.String()
}

func createConfigMapList(t *testing.T, configmaps string) *v1.ConfigMapList {
	var cmlistc CMListConfig
	cmlistc.List = configmaps
	cmlisttmpl := template.New("configmaplist")
	cmlisttmpl, err3 := cmlisttmpl.Parse(cmlisttemplate)
	assert.Nil(t, err3)
	var b1 bytes.Buffer
	err4 := cmlisttmpl.Execute(&b1, cmlistc)
	assert.Nil(t, err4)

	decode := scheme.Codecs.UniversalDeserializer().Decode

	obj, _, err2 := decode(b1.Bytes(), nil, nil)
	assert.Nil(t, err2)
	return obj.(*v1.ConfigMapList)
}
