// +build unit

package pipelinescheduler_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/pipelinescheduler"
	"github.com/jenkins-x/jx/pkg/pipelinescheduler/testhelpers"
	"github.com/stretchr/testify/assert"

	"github.com/pborman/uuid"
)

func TestBuild(t *testing.T) {

	org := uuid.New()
	leaf1 := &pipelinescheduler.SchedulerLeaf{
		Org:           org,
		Repo:          uuid.New(),
		SchedulerSpec: testhelpers.CompleteScheduler(),
	}
	leaf2 := &pipelinescheduler.SchedulerLeaf{
		Org:           org,
		Repo:          uuid.New(),
		SchedulerSpec: testhelpers.CompleteScheduler(),
	}
	leaves := []*pipelinescheduler.SchedulerLeaf{
		leaf1,
		leaf2,
	}
	// TODO test plugins
	cfg, _, err := pipelinescheduler.BuildProwConfig(leaves)
	assert.NoError(t, err)
	assert.Len(t, cfg.Postsubmits, 2)
	assert.Len(t, cfg.Presubmits, 2)
	assert.Equal(t, &cfg.Presubmits[fmt.Sprintf("%s/%s", org, leaf1.Repo)][0].Name, leaf1.Presubmits.Items[0].Name)
}

func TestRepo(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	testhelpers.BuildAndValidateProwConfig(t, filepath.Join(wd, "test_data", "repo"), "config.yaml", "",
		[]testhelpers.SchedulerFile{
			{
				Filenames: []string{"repo.yaml"},
				Org:       "acme",
				Repo:      "dummy",
			},
		})
}

func TestMultipleContexts(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	testhelpers.BuildAndValidateProwConfig(t, filepath.Join(wd, "test_data", "multiple_contexts"), "config.yaml", "",
		[]testhelpers.SchedulerFile{
			{
				Filenames: []string{"repo.yaml"},
				Org:       "acme",
				Repo:      "dummy",
			},
		})
}

func TestWithParent(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	testhelpers.BuildAndValidateProwConfig(t, filepath.Join(wd, "test_data", "with_parent"), "config.yaml",
		"plugins.yaml", []testhelpers.SchedulerFile{
			{
				Filenames: []string{"parent.yaml", "repo.yaml"},
				Org:       "acme",
				Repo:      "dummy",
			},
		})
}

func TestNoPostSubmitsWithParent(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	testhelpers.BuildAndValidateProwConfig(t, filepath.Join(wd, "test_data", "no_postsubmits_with_parent"), "config.yaml",
		"plugins.yaml", []testhelpers.SchedulerFile{
			{
				Filenames: []string{"parent.yaml", "repo.yaml"},
				Org:       "acme",
				Repo:      "dummy",
			},
		})
}

func TestPolicyWithParent(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	testhelpers.BuildAndValidateProwConfig(t, filepath.Join(wd, "test_data", "policy_with_parent"), "config.yaml",
		"plugins.yaml", []testhelpers.SchedulerFile{
			{
				Filenames: []string{"parent.yaml", "repo.yaml"},
				Org:       "acme",
				Repo:      "dummy",
			},
		})
}

func TestMergerWithParent(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	testhelpers.BuildAndValidateProwConfig(t, filepath.Join(wd, "test_data", "merger_with_parent"), "config.yaml",
		"plugins.yaml", []testhelpers.SchedulerFile{
			{
				Filenames: []string{"parent.yaml", "repo.yaml"},
				Org:       "acme",
				Repo:      "dummy",
			},
		})
}

func TestMergerWithMergeMethod(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	testhelpers.BuildAndValidateProwConfig(t, filepath.Join(wd, "test_data", "merger_with_mergemethod"), "config.yaml",
		"plugins.yaml", []testhelpers.SchedulerFile{
			{
				Filenames: []string{"parent.yaml", "repo.yaml"},
				Org:       "acme",
				Repo:      "dummy",
			},
		})
}

func TestOnlyWithParent(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	testhelpers.BuildAndValidateProwConfig(t, filepath.Join(wd, "test_data", "only_with_parent"), "config.yaml",
		"plugins.yaml", []testhelpers.SchedulerFile{
			{
				Filenames: []string{"parent.yaml"},
				Org:       "acme",
				Repo:      "dummy",
			},
		})
}

func TestOnlyPluginsFromRepo(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	testhelpers.BuildAndValidateProwConfig(t, filepath.Join(wd, "test_data", "only_plugins_from_repo"), "",
		"plugins.yaml", []testhelpers.SchedulerFile{
			{
				Filenames: []string{"parent.yaml", "repo.yaml"},
				Org:       "acme",
				Repo:      "dummy",
			},
		})
}

func TestOnlyPluginsJustFromParent(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	testhelpers.BuildAndValidateProwConfig(t, filepath.Join(wd, "test_data", "only_plugins_from_parent"), "",
		"plugins.yaml", []testhelpers.SchedulerFile{
			{
				Filenames: []string{"parent.yaml", "repo.yaml"},
				Org:       "acme",
				Repo:      "dummy",
			},
		})
}

func TestOnlyPluginsMixFromParentAndRepo(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	testhelpers.BuildAndValidateProwConfig(t, filepath.Join(wd, "test_data", "only_plugins"), "",
		"plugins.yaml", []testhelpers.SchedulerFile{
			{
				Filenames: []string{"parent.yaml", "repo.yaml"},
				Org:       "acme",
				Repo:      "dummy",
			},
		})
}
