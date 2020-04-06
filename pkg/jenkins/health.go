package jenkins

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

const (
	pingTimeout = 2 * time.Second
)

// CheckHealth checks the health of Jenkins server using the login URL
func CheckHealth(url string, healthTimeout time.Duration) error {
	endpoint := JenkinsLoginURL(url)

	if ping(endpoint) == nil {
		return nil
	}
	log.Logger().Infof("waiting up to %s for the Jenkins server to be healthy at URL %s", util.ColorInfo(healthTimeout.String()), util.ColorInfo(endpoint))
	err := util.Retry(healthTimeout, func() error {
		return ping(endpoint)
	})

	if err != nil {
		return errors.Wrapf(err, "pinging Jenkins server %q", endpoint)
	}
	return nil
}

func ping(url string) error {
	client := http.Client{
		Timeout: pingTimeout,
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return errors.Wrapf(err, "building ping request for URL %q", url)
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "executing ping request against URL %q", url)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping status: %s", resp.Status)
	}
	return nil
}
