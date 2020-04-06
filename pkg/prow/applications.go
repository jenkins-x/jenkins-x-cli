package prow

import (
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// AddDummyApplication creates the dummy prow jenkins app
func AddDummyApplication(client kubernetes.Interface, devNamespace string, settings *v1.TeamSettings) error {

	var err error
	log.Logger().Infof("Setting up prow config into namespace %s", util.ColorInfo(devNamespace))

	// create initial configmaps if they don't already exist, use a dummy repo so tide doesn't start scanning all github
	_, err = client.CoreV1().ConfigMaps(devNamespace).Get("config", v12.GetOptions{})
	if err != nil {
		err = AddApplication(client, []string{"jenkins-x/dummy"}, devNamespace, "base", settings)
		if err != nil {
			return errors.Wrap(err, "adding dummy application")
		}
	}
	return nil
}
