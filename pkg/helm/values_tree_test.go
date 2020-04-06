// +build unit

package helm_test

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/secreturl/localvault"
	"github.com/stretchr/testify/assert"
)

func TestValuesTree(t *testing.T) {
	t.Parallel()
	vaultClient := localvault.NewFileSystemClient(path.Join("test_data", "local_vault_files"))
	dir, err := createFiles(map[string]string{
		"cheese/values.yaml": "foo: bar",
		"meat/ham/values.yaml": `foo: 
  bar: baz`,
	})
	expectedOutput := `cheese:
  foo: bar
meat:
  ham:
    foo:
      bar: baz
`
	defer func() {
		err := os.RemoveAll(dir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	result, _, err := helm.GenerateValues(config.NewRequirementsConfig(), nil, dir, nil, true, vaultClient)
	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, string(result))
}

func TestValuesTreeWithExistingFile(t *testing.T) {
	t.Parallel()
	vaultClient := localvault.NewFileSystemClient(path.Join("test_data", "local_vault_files"))
	dir, err := createFiles(map[string]string{
		"values.yaml":        "people: pete",
		"cheese/values.yaml": "foo: bar",
		"meat/ham/values.yaml": `foo: 
  bar: baz`,
	})
	expectedOutput := `cheese:
  foo: bar
meat:
  ham:
    foo:
      bar: baz
people: pete
`
	defer func() {
		err := os.RemoveAll(dir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	result, _, err := helm.GenerateValues(config.NewRequirementsConfig(), nil, dir, nil, true, vaultClient)
	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, string(result))
}

func TestValuesTreeWithFileRefs(t *testing.T) {
	t.Parallel()
	vaultClient := localvault.NewFileSystemClient(path.Join("test_data", "local_vault_files"))
	dir, err := createFiles(map[string]string{
		"milk/values.yaml": `foo:
  bar:
    full-fat.xml:
      `,
		"milk/full-fat.xml": `<milk>
    <creamy />
</milk>`,
	})
	expectedOutput := `milk:
  foo:
    bar:
      full-fat.xml: |-
        <milk>
            <creamy />
        </milk>
`
	defer func() {
		err := os.RemoveAll(dir)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	result, _, err := helm.GenerateValues(config.NewRequirementsConfig(), nil, dir, nil, true, vaultClient)
	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, string(result))
}

func createFiles(files map[string]string) (string, error) {
	dir, err := ioutil.TempDir("", "values_tree_test")
	if err != nil {
		return "", err
	}
	for path, value := range files {
		subDir, _ := filepath.Split(path)
		err := os.MkdirAll(filepath.Join(dir, subDir), 0700)
		if err != nil {
			return "", err
		}
		err = ioutil.WriteFile(filepath.Join(dir, path), []byte(value), 0755)
		if err != nil {
			return "", err
		}
	}
	return dir, nil
}
