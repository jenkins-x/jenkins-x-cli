// +build unit

package kube

import (
	"fmt"
	"os"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_compareBuildLocks(t *testing.T) {
	time1 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	time2 := time.Date(2000, 1, 1, 0, 0, 0, 200000000, time.UTC).Format(time.RFC3339Nano)
	time3 := time.Date(2000, 1, 1, 0, 0, 0, 210000000, time.UTC).Format(time.RFC3339Nano)
	examples := []struct {
		name string
		old  map[string]string
		new  map[string]string
		ret  map[string]string
		err  bool
	}{{
		"same build",
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "123",
			"pod":        "build-pod-123",
			"timestamp":  time2,
		},
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "123",
			"pod":        "build-pod-123",
			"timestamp":  time2,
		},
		nil,
		false,
	}, {
		"same build but different pod (for some reason)",
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "123",
			"pod":        "build-pod-123",
			"timestamp":  time2,
		},
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "123",
			"pod":        "other-pod-123",
			"timestamp":  time2,
		},
		nil,
		true,
	}, {
		"lower build",
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "101",
			"pod":        "build-pod-101",
			"timestamp":  time2,
		},
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "99",
			"pod":        "build-pod-99",
			"timestamp":  time1,
		},
		nil,
		true,
	}, {
		"higher build",
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "101",
			"pod":        "build-pod-101",
			"timestamp":  time1,
		},
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "103",
			"pod":        "build-pod-103",
			"timestamp":  time2,
		},
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "103",
			"pod":        "build-pod-103",
			"timestamp":  time2,
		},
		false,
	}, {
		"higher build but lower timestamp",
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "101",
			"pod":        "build-pod-101",
			"timestamp":  time3,
		},
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "103",
			"pod":        "build-pod-103",
			"timestamp":  time2,
		},
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "103",
			"pod":        "build-pod-103",
			"timestamp":  time3,
		},
		false,
	}, {
		"other build, same timestamp",
		map[string]string{
			"owner":      "other-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "111",
			"pod":        "build-pod-111",
			"timestamp":  time2,
		},
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "123",
			"pod":        "other-pod-123",
			"timestamp":  time2,
		},
		nil,
		true,
	}, {
		"other build, lower timestamp",
		map[string]string{
			"owner":      "my-owner",
			"repository": "other-repository",
			"branch":     "my-branch",
			"build":      "111",
			"pod":        "build-pod-111",
			"timestamp":  time2,
		},
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "123",
			"pod":        "other-pod-123",
			"timestamp":  time1,
		},
		nil,
		true,
	}, {
		"other build, higher timestamp",
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "other-branch",
			"build":      "111",
			"pod":        "build-pod-111",
			"timestamp":  time2,
		},
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "123",
			"pod":        "other-pod-123",
			"timestamp":  time3,
		},
		map[string]string{
			"owner":      "my-owner",
			"repository": "my-repository",
			"branch":     "my-branch",
			"build":      "123",
			"pod":        "other-pod-123",
			"timestamp":  time3,
		},
		false,
	}}
	for _, example := range examples {
		ret, err := compareBuildLocks(example.old, example.new)
		assert.Equal(t, example.ret, ret, example.name)
		if example.err {
			assert.Error(t, err, example.name)
		} else {
			assert.NoError(t, err, example.name)
		}
	}
}

// buildLock_Client creates a fake client with a fake tekton deployment
func buildLock_Client(t *testing.T) *fake.Clientset {
	client := fake.NewSimpleClientset()
	_, err := client.AppsV1().Deployments("jx").Create(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentTektonController,
			Namespace: "jx",
		},
	})
	require.NoError(t, err)
	return client
}

// buildLock_CountWatch count watchers for synchronization reasons
func buildLock_CountWatch(client *fake.Clientset) chan int {
	c := make(chan int, 100)
	count := 0
	client.PrependWatchReactor("*", func(action ktesting.Action) (handled bool, ret watch.Interface, err error) {
		count++
		c <- count
		return false, nil, nil
	})
	return c
}

var buildLock_UID int = 1 << 20 // the pid of out fake pods
// buildLock_Pod creates a running pod, looking close enough to a pipeline pod
func buildLock_Pod(t *testing.T, client kubernetes.Interface, owner, repository, branch, build string) *v1.Pod {
	buildLock_UID++
	pod, err := client.CoreV1().Pods("jx").Create(&v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("pipeline-%s-%s-%s-%s", owner, repository, branch, build),
			Namespace: "jx",
			Labels: map[string]string{
				"owner":                   owner,
				"repository":              repository,
				"branch":                  branch,
				"build":                   build,
				"jenkins.io/pipelineType": "build",
			},
			UID: types.UID(fmt.Sprintf("%d", buildLock_UID)),
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	})
	require.NoError(t, err)
	return pod
}

// buildLock_Lock creates a lock
func buildLock_Lock(t *testing.T, client kubernetes.Interface, namespace, owner, repository, branch, build string, minutes int, expires time.Duration) *v1.ConfigMap {
	exp := time.Now().UTC().Add(expires).Format(time.RFC3339Nano)
	lock, err := client.CoreV1().ConfigMaps("jx").Create(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-lock-my-namespace",
			Namespace: "jx",
			Labels: map[string]string{
				"namespace":         namespace,
				"owner":             owner,
				"repository":        repository,
				"branch":            branch,
				"build":             build,
				"jenkins-x.io/kind": "build-lock",
			},
			Annotations: map[string]string{
				"expires": exp,
			},
		},
		Data: map[string]string{
			"namespace":  namespace,
			"owner":      owner,
			"repository": repository,
			"branch":     branch,
			"build":      build,
			"timestamp":  buildLock_Timestamp(minutes),
			"expires":    exp,
		},
	})
	require.NoError(t, err)
	return lock
}

// buildLock_LockFromPod creates a lock that matches a pod
func buildLock_LockFromPod(t *testing.T, client kubernetes.Interface, namespace string, pod *v1.Pod, minutes int) *v1.ConfigMap {
	lock, err := client.CoreV1().ConfigMaps("jx").Create(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-lock-my-namespace",
			Namespace: "jx",
			Labels: map[string]string{
				"namespace":         namespace,
				"owner":             pod.Labels["owner"],
				"repository":        pod.Labels["repository"],
				"branch":            pod.Labels["branch"],
				"build":             pod.Labels["build"],
				"jenkins-x.io/kind": "build-lock",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       pod.Name,
				UID:        pod.UID,
			}},
		},
		Data: map[string]string{
			"namespace":  namespace,
			"owner":      pod.Labels["owner"],
			"repository": pod.Labels["repository"],
			"branch":     pod.Labels["branch"],
			"build":      pod.Labels["build"],
			"pod":        pod.Name,
			"timestamp":  buildLock_Timestamp(minutes),
		},
	})
	require.NoError(t, err)
	return lock
}

// buildLock_Timestamp create the timestamp for a lock, now plus or minus some minutes
func buildLock_Timestamp(minutes int) string {
	now := time.Now().UTC()
	now = now.Add(time.Duration(minutes) * time.Minute)
	return now.Format(time.RFC3339Nano)
}

// buildLock_Env prepares the environment for calling AcquireBuildLock
// returns a defer function to restore the environment
func buildLock_Env(t *testing.T, owner, repository, branch, build string, interpret bool) func() {
	v := ""
	if interpret {
		v = "true"
	}
	env := map[string]string{
		"REPO_OWNER":            owner,
		"REPO_NAME":             repository,
		"BRANCH_NAME":           branch,
		"BUILD_NUMBER":          build,
		"JX_INTERPRET_PIPELINE": v,
	}
	old := map[string]string{}
	for k, v := range env {
		value, ok := os.LookupEnv(k)
		if ok {
			old[k] = value
		}
		var err error
		if v == "" {
			err = os.Unsetenv(k)
		} else {
			err = os.Setenv(k, v)
		}
		require.NoError(t, err)
	}
	return func() {
		for k := range env {
			v, ok := old[k]
			var err error
			if ok {
				err = os.Setenv(k, v)
			} else {
				err = os.Unsetenv(k)
			}
			assert.NoError(t, err)
		}
	}
}

// buildLock_Acquire calls AcquireBuildLock with arguments
// returns a defer function to restore the environment
// returns a chan that is filled once AcquireBuildLock returns
// its item will perform some check and call the callback
// its item is nil on timeout
func buildLock_Acquire(t *testing.T, client kubernetes.Interface, namespace, owner, repository, branch, build string, fails bool) (func(), chan func()) {
	c := make(chan func(), 2)
	clean := buildLock_Env(t, owner, repository, branch, build, true)
	go func() {
		callback, err := AcquireBuildLock(client, "jx", namespace)
		c <- func() {
			if !fails {
				require.NoError(t, err)
				assert.NoError(t, callback())
			} else {
				require.Error(t, err)
			}
		}
	}()
	go func() {
		time.Sleep(time.Duration(5) * time.Second)
		c <- nil
	}()
	return clean, c
}

// buildLock_AcquireFromPod calls AcquireBuildLock with arguments matching a pod
// returns a defer function to restore the environment
// returns a chan that is filled once AcquireBuildLock returns
// its item will perform some check and call the callback
// its item is nil on timeout
func buildLock_AcquireFromPod(t *testing.T, client kubernetes.Interface, namespace string, pod *v1.Pod, fails bool) (func(), chan func()) {
	c := make(chan func(), 2)
	clean := buildLock_Env(t, pod.Labels["owner"], pod.Labels["repository"], pod.Labels["branch"], pod.Labels["build"], false)
	go func() {
		callback, err := AcquireBuildLock(client, "jx", namespace)
		c <- func() {
			if !fails {
				require.NoError(t, err)
				assert.NoError(t, callback())
			} else {
				require.Error(t, err)
			}
		}
	}()
	go func() {
		time.Sleep(time.Duration(5) * time.Second)
		c <- nil
	}()
	return clean, c
}

func buildLock_AssertNoLock(t *testing.T, client kubernetes.Interface, namespace string) {
	lock, err := client.CoreV1().ConfigMaps("jx").Get("jx-lock-"+namespace, metav1.GetOptions{})
	assert.Nil(t, lock)
	if assert.Error(t, err) {
		require.IsType(t, &errors.StatusError{}, err)
		status := err.(*errors.StatusError)
		require.Equal(t, metav1.StatusReasonNotFound, status.Status().Reason)
	}
}

// buildLock_AssertLock checks if the lock configmap is correct
func buildLock_AssertLock(t *testing.T, client kubernetes.Interface, namespace, owner, repository, branch, build string) {
	lock, err := client.CoreV1().ConfigMaps("jx").Get("jx-lock-"+namespace, metav1.GetOptions{})
	require.NoError(t, err)
	if assert.NotNil(t, lock) {
		assert.Equal(t, "build-lock", lock.Labels["jenkins-x.io/kind"])
		assert.Equal(t, namespace, lock.Labels["namespace"])
		assert.Equal(t, owner, lock.Labels["owner"])
		assert.Equal(t, repository, lock.Labels["repository"])
		assert.Equal(t, branch, lock.Labels["branch"])
		assert.Equal(t, build, lock.Labels["build"])
		assert.Empty(t, lock.OwnerReferences)
		assert.Equal(t, namespace, lock.Data["namespace"])
		assert.Equal(t, owner, lock.Data["owner"])
		assert.Equal(t, repository, lock.Data["repository"])
		assert.Equal(t, branch, lock.Data["branch"])
		assert.Equal(t, build, lock.Data["build"])
		assert.Equal(t, "", lock.Data["pod"])
		ts, err := time.Parse(time.RFC3339Nano, lock.Data["timestamp"])
		if assert.NoError(t, err) {
			assert.True(t, ts.Before(time.Now().Add(time.Minute)))
			assert.True(t, ts.After(time.Now().Add(time.Duration(-1)*time.Minute)))
		}
		ts, err = time.Parse(time.RFC3339Nano, lock.Annotations["expires"])
		if assert.NoError(t, err) {
			// tighter check to be sure that expires is updated
			assert.True(t, ts.Before(time.Now().Add(buildLockExpires+time.Duration(1500)*time.Millisecond)))
			assert.True(t, ts.After(time.Now().Add(buildLockExpires+time.Duration(-1500)*time.Millisecond)))
			assert.Equal(t, lock.Annotations["expires"], lock.Data["expires"])
		}
	}
}

// buildLock_AssertLockFromPod checks if the lock configmap is matching the given pod
func buildLock_AssertLockFromPod(t *testing.T, client kubernetes.Interface, namespace string, pod *v1.Pod) {
	lock, err := client.CoreV1().ConfigMaps("jx").Get("jx-lock-"+namespace, metav1.GetOptions{})
	require.NoError(t, err)
	if assert.NotNil(t, lock) {
		assert.Equal(t, "build-lock", lock.Labels["jenkins-x.io/kind"])
		assert.Equal(t, namespace, lock.Labels["namespace"])
		assert.Equal(t, pod.Labels["owner"], lock.Labels["owner"])
		assert.Equal(t, pod.Labels["repository"], lock.Labels["repository"])
		assert.Equal(t, pod.Labels["branch"], lock.Labels["branch"])
		assert.Equal(t, pod.Labels["build"], lock.Labels["build"])
		assert.Equal(t, []metav1.OwnerReference{{
			APIVersion: pod.APIVersion,
			Kind:       pod.Kind,
			Name:       pod.Name,
			UID:        pod.UID,
		}}, lock.OwnerReferences)
		assert.Equal(t, "", lock.Annotations["expires"])
		assert.Equal(t, namespace, lock.Data["namespace"])
		assert.Equal(t, pod.Labels["owner"], lock.Data["owner"])
		assert.Equal(t, pod.Labels["repository"], lock.Data["repository"])
		assert.Equal(t, pod.Labels["branch"], lock.Data["branch"])
		assert.Equal(t, pod.Labels["build"], lock.Data["build"])
		assert.Equal(t, pod.Name, lock.Data["pod"])
		ts, err := time.Parse(time.RFC3339Nano, lock.Data["timestamp"])
		if assert.NoError(t, err) {
			assert.True(t, ts.Before(time.Now().Add(time.Minute)))
			assert.True(t, ts.After(time.Now().Add(time.Duration(-1)*time.Minute)))
		}
		assert.Equal(t, "", lock.Data["expires"])
	}
}

func TestAcquireBuildLock(t *testing.T) {
	// just acquire a lock when no lock exists
	client := buildLock_Client(t)
	pod := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "13")
	clean, channel := buildLock_AcquireFromPod(t, client, "my-namespace", pod, false)
	defer clean()
	callback := <-channel
	require.NotNil(t, callback, "timeout")
	buildLock_AssertLockFromPod(t, client, "my-namespace", pod)
	callback()
	buildLock_AssertNoLock(t, client, "my-namespace")
}

func TestAcquireBuildLock_interpret(t *testing.T) {
	// acquire a lock with an intepreted pipeline
	client := buildLock_Client(t)
	clean := buildLock_Env(t, "my-owner", "my-repository", "my-branch", "13", true)
	defer clean()
	channel := make(chan func(), 2)
	go func() {
		callback, err := AcquireBuildLock(client, "jx", "my-namespace")
		channel <- func() {
			require.NoError(t, err)
			assert.NoError(t, callback())
		}
	}()
	go func() {
		time.Sleep(time.Duration(5) * time.Second)
		channel <- nil
	}()

	callback := <-channel
	require.NotNil(t, callback, "timeout")
	buildLock_AssertLock(t, client, "my-namespace", "my-owner", "my-repository", "my-branch", "13")
	callback()
	buildLock_AssertNoLock(t, client, "my-namespace")
}

func TestAcquireBuildLock_invalidLock(t *testing.T) {
	// acquire a lock when the previous lock is invalid
	client := buildLock_Client(t)
	previous := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "42")
	lock := buildLock_LockFromPod(t, client, "my-namespace", previous, -42)
	lock.Labels["jenkins-x.io/kind"] = "other-lock"
	_, err := client.CoreV1().ConfigMaps("jx").Update(lock)
	require.NoError(t, err)

	pod := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "13")
	clean, channel := buildLock_AcquireFromPod(t, client, "my-namespace", pod, false)
	defer clean()
	callback := <-channel
	require.NotNil(t, callback, "timeout")
	buildLock_AssertLockFromPod(t, client, "my-namespace", pod)
	callback()
	buildLock_AssertNoLock(t, client, "my-namespace")
}

func TestAcquireBuildLock_previousNotFound(t *testing.T) {
	// acquire a lock when the locking pod does not exist
	client := buildLock_Client(t)
	previous := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "42")
	buildLock_LockFromPod(t, client, "my-namespace", previous, 42)
	err := client.CoreV1().Pods("jx").Delete(previous.Name, &metav1.DeleteOptions{})
	require.NoError(t, err)

	pod := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "13")
	clean, channel := buildLock_AcquireFromPod(t, client, "my-namespace", pod, false)
	defer clean()
	callback := <-channel
	require.NotNil(t, callback, "timeout")
	buildLock_AssertLockFromPod(t, client, "my-namespace", pod)
	callback()
	buildLock_AssertNoLock(t, client, "my-namespace")
}

func TestAcquireBuildLock_previousFinished(t *testing.T) {
	// acquire a lock when the locking pod has finished
	client := buildLock_Client(t)
	previous := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "42")
	buildLock_LockFromPod(t, client, "my-namespace", previous, 42)
	previous.Status.Phase = v1.PodFailed
	_, err := client.CoreV1().Pods("jx").Update(previous)
	require.NoError(t, err)

	pod := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "13")
	clean, channel := buildLock_AcquireFromPod(t, client, "my-namespace", pod, false)
	defer clean()
	callback := <-channel
	require.NotNil(t, callback, "timeout")
	buildLock_AssertLockFromPod(t, client, "my-namespace", pod)
	callback()
	buildLock_AssertNoLock(t, client, "my-namespace")
}

func TestAcquireBuildLock_expired(t *testing.T) {
	// acquire a lock when the previous lock has expired
	client := buildLock_Client(t)
	buildLock_Lock(t, client, "my-namespace", "my-owner", "my-repository", "my-branch", "42", 42, time.Duration(-1)*time.Minute)

	pod := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "13")
	clean, channel := buildLock_AcquireFromPod(t, client, "my-namespace", pod, false)
	defer clean()
	callback := <-channel
	require.NotNil(t, callback, "timeout")
	buildLock_AssertLockFromPod(t, client, "my-namespace", pod)
	callback()
	buildLock_AssertNoLock(t, client, "my-namespace")
}

func TestAcquireBuildLock_higherRuns(t *testing.T) {
	// fails at acquiring the lock because an higher build is running
	client := buildLock_Client(t)
	previous := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "42")
	buildLock_LockFromPod(t, client, "my-namespace", previous, -42)

	pod := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "13")
	clean, channel := buildLock_AcquireFromPod(t, client, "my-namespace", pod, true)
	defer clean()
	callback := <-channel
	callback()
}

func TestAcquireBuildLock_laterRuns(t *testing.T) {
	// fails at acquiring the lock because a later build is running
	client := buildLock_Client(t)
	previous := buildLock_Pod(t, client, "other-owner", "other-repository", "other-branch", "42")
	buildLock_LockFromPod(t, client, "my-namespace", previous, 42)

	pod := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "13")
	clean, channel := buildLock_AcquireFromPod(t, client, "my-namespace", pod, true)
	defer clean()
	callback := <-channel
	callback()
}

func TestAcquireBuildLock_waitLowerPodDeleted(t *testing.T) {
	// wait for a lower build to be deleted
	client := buildLock_Client(t)
	counter := buildLock_CountWatch(client)
	previous := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "11")
	old := buildLock_LockFromPod(t, client, "my-namespace", previous, 11)
	// should update the lock
	pod := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "13")
	clean, channel := buildLock_AcquireFromPod(t, client, "my-namespace", pod, false)
	defer clean()
	// wait for AcquireBuildLock to be waiting
	for {
		count := 0
		select {
		case count = <-counter:
		case callback := <-channel:
			require.NotNil(t, callback, "timeout")
			assert.Fail(t, "TestAcquireBuildLock returned")
			callback()
			return
		}
		if count == 2 {
			break
		}
	}
	// check the lock
	lock, err := client.CoreV1().ConfigMaps("jx").Get(old.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, old.ObjectMeta, lock.ObjectMeta)
	assert.Equal(t, "my-namespace", lock.Data["namespace"])
	assert.Equal(t, "my-owner", lock.Data["owner"])
	assert.Equal(t, "my-repository", lock.Data["repository"])
	assert.Equal(t, "my-branch", lock.Data["branch"])
	assert.Equal(t, "13", lock.Data["build"])
	assert.Equal(t, pod.Name, lock.Data["pod"])
	assert.Equal(t, old.Data["timestamp"], lock.Data["timestamp"])
	// should acquire the lock
	err = client.CoreV1().Pods("jx").Delete(previous.Name, &metav1.DeleteOptions{})
	require.NoError(t, err)
	callback := <-channel
	require.NotNil(t, callback, "timeout")
	buildLock_AssertLockFromPod(t, client, "my-namespace", pod)
	callback()
	buildLock_AssertNoLock(t, client, "my-namespace")
}

func TestAcquireBuildLock_waitLowerLockDeleted(t *testing.T) {
	// wait for a lower build lock to be deleted
	client := buildLock_Client(t)
	counter := buildLock_CountWatch(client)
	previous := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "11")
	old := buildLock_LockFromPod(t, client, "my-namespace", previous, 11)
	// should update the lock
	pod := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "13")
	clean, channel := buildLock_AcquireFromPod(t, client, "my-namespace", pod, false)
	defer clean()
	// wait for AcquireBuildLock to be waiting
	for {
		count := 0
		select {
		case count = <-counter:
		case callback := <-channel:
			require.NotNil(t, callback, "timeout")
			assert.Fail(t, "TestAcquireBuildLock returned")
			callback()
			return
		}
		if count == 2 {
			break
		}
	}
	// check the lock
	lock, err := client.CoreV1().ConfigMaps("jx").Get(old.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, old.ObjectMeta, lock.ObjectMeta)
	assert.Equal(t, "my-namespace", lock.Data["namespace"])
	assert.Equal(t, "my-owner", lock.Data["owner"])
	assert.Equal(t, "my-repository", lock.Data["repository"])
	assert.Equal(t, "my-branch", lock.Data["branch"])
	assert.Equal(t, "13", lock.Data["build"])
	assert.Equal(t, pod.Name, lock.Data["pod"])
	assert.Equal(t, old.Data["timestamp"], lock.Data["timestamp"])
	// should acquire the lock
	err = client.CoreV1().ConfigMaps("jx").Delete(old.Name, &metav1.DeleteOptions{})
	require.NoError(t, err)
	callback := <-channel
	require.NotNil(t, callback, "timeout")
	buildLock_AssertLockFromPod(t, client, "my-namespace", pod)
	callback()
	buildLock_AssertNoLock(t, client, "my-namespace")
}

func TestAcquireBuildLock_waitEarlierFinished(t *testing.T) {
	// wait for a lower build to finish
	client := buildLock_Client(t)
	counter := buildLock_CountWatch(client)
	previous := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "11")
	old := buildLock_LockFromPod(t, client, "my-namespace", previous, 11)
	// should update the lock
	clean, channel := buildLock_Acquire(t, client, "my-namespace", "my-owner", "my-repository", "my-branch", "13", false)
	defer clean()
	// wait for AcquireBuildLock to be waiting
	for {
		count := 0
		select {
		case count = <-counter:
		case callback := <-channel:
			require.NotNil(t, callback, "timeout")
			assert.Fail(t, "TestAcquireBuildLock returned")
			callback()
			return
		}
		if count == 1 {
			break
		}
	}
	// check the lock
	lock, err := client.CoreV1().ConfigMaps("jx").Get(old.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, old.ObjectMeta, lock.ObjectMeta)
	assert.Equal(t, "my-namespace", lock.Data["namespace"])
	assert.Equal(t, "my-owner", lock.Data["owner"])
	assert.Equal(t, "my-repository", lock.Data["repository"])
	assert.Equal(t, "my-branch", lock.Data["branch"])
	assert.Equal(t, "13", lock.Data["build"])
	assert.Equal(t, "", lock.Data["pod"])
	assert.Equal(t, old.Data["timestamp"], lock.Data["timestamp"])
	// should acquire the lock
	previous.Status.Phase = v1.PodSucceeded
	_, err = client.CoreV1().Pods("jx").Update(previous)
	require.NoError(t, err)
	callback := <-channel
	require.NotNil(t, callback, "timeout")
	buildLock_AssertLock(t, client, "my-namespace", "my-owner", "my-repository", "my-branch", "13")
	callback()
	buildLock_AssertNoLock(t, client, "my-namespace")
}

func TestAcquireBuildLock_waitLowerExpired(t *testing.T) {
	// wait for a lock to expire
	client := buildLock_Client(t)
	counter := buildLock_CountWatch(client)
	old := buildLock_Lock(t, client, "my-namespace", "my-owner", "my-repository", "my-branch", "11", 11, time.Duration(2)*time.Second)
	// should update the lock
	pod := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "13")
	clean, channel := buildLock_AcquireFromPod(t, client, "my-namespace", pod, false)
	defer clean()
	// wait for AcquireBuildLock to be waiting
	for {
		count := 0
		select {
		case count = <-counter:
		case callback := <-channel:
			require.NotNil(t, callback, "timeout")
			assert.Fail(t, "TestAcquireBuildLock returned")
			callback()
			return
		}
		if count == 1 {
			break
		}
	}
	// check the lock
	lock, err := client.CoreV1().ConfigMaps("jx").Get(old.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, old.ObjectMeta, lock.ObjectMeta)
	assert.Equal(t, "my-namespace", lock.Data["namespace"])
	assert.Equal(t, "my-owner", lock.Data["owner"])
	assert.Equal(t, "my-repository", lock.Data["repository"])
	assert.Equal(t, "my-branch", lock.Data["branch"])
	assert.Equal(t, "13", lock.Data["build"])
	assert.Equal(t, pod.Name, lock.Data["pod"])
	assert.Equal(t, old.Data["timestamp"], lock.Data["timestamp"])
	// should acquire the lock after 2 seconds
	callback := <-channel
	require.NotNil(t, callback, "timeout")
	buildLock_AssertLockFromPod(t, client, "my-namespace", pod)
	callback()
	buildLock_AssertNoLock(t, client, "my-namespace")
}

func TestAcquireBuildLock_waitButHigher(t *testing.T) {
	// wait for a lower run to finish, but an higher run appears
	client := buildLock_Client(t)
	counter := buildLock_CountWatch(client)
	previous := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "11")
	old := buildLock_LockFromPod(t, client, "my-namespace", previous, -11)
	// should update the lock
	pod := buildLock_Pod(t, client, "my-owner", "my-repository", "my-branch", "13")
	clean, channel := buildLock_AcquireFromPod(t, client, "my-namespace", pod, true)
	defer clean()
	// wait for AcquireBuildLock to be waiting
	for {
		count := 0
		select {
		case count = <-counter:
		case callback := <-channel:
			require.NotNil(t, callback, "timeout")
			assert.Fail(t, "TestAcquireBuildLock returned")
			callback()
			return
		}
		if count == 2 {
			break
		}
	}
	// check the lock
	lock, err := client.CoreV1().ConfigMaps("jx").Get(old.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, old.ObjectMeta, lock.ObjectMeta)
	assert.Equal(t, "my-namespace", lock.Data["namespace"])
	assert.Equal(t, "my-owner", lock.Data["owner"])
	assert.Equal(t, "my-repository", lock.Data["repository"])
	assert.Equal(t, "my-branch", lock.Data["branch"])
	assert.Equal(t, "13", lock.Data["build"])
	ts, err := time.Parse(time.RFC3339Nano, lock.Data["timestamp"])
	if assert.NoError(t, err) {
		assert.True(t, ts.Before(time.Now().Add(time.Minute)))
		assert.True(t, ts.After(time.Now().Add(time.Duration(-1)*time.Minute)))
	}
	// update the lock and expect failure
	lock.Data["build"] = "21"
	_, err = client.CoreV1().ConfigMaps("jx").Update(lock)
	require.NoError(t, err)
	callback := <-channel
	require.NotNil(t, callback, "timeout")
	callback()
}
