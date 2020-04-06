// +build integration

package create_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/deletecmd"

	"github.com/jenkins-x/jx/pkg/cmd/clients"

	"github.com/stretchr/testify/assert"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/create"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
)

// PLM: This passes if it's run against Fake Kubernetes
func TestCreateUser(t *testing.T) {
	factory := clients.NewFactory()
	commonOpts := opts.NewCommonOptionsWithFactory(factory)
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	login := r1.Int()
	o := create.CreateUserOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: &commonOpts,
		},
		UserSpec: v1.UserDetails{
			Login: fmt.Sprintf("%d", login),
			Email: "porterja31@github.com",
			Name:  "porterja31",
		},
	}
	defer func() {
		do := deletecmd.DeleteUserOptions{
			CommonOptions: &commonOpts,
			Confirm:       true,
		}
		do.Args = []string{fmt.Sprintf("%d", login)}
		do.BatchMode = true
		err := do.Run()
		if err != nil {
			t.Logf("Error running jx delete user %d %v", login, err)
		}
	}()
	err := o.Run()
	assert.NoError(t, err)
}
