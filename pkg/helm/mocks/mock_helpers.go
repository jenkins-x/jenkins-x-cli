package helm_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/petergtz/pegomock"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

// StubFetchChart stubs out the FetchChart operations on MockHelmer creating the chart.
func StubFetchChart(name string, version string, repo string, chartToCreate *chart.Chart, mockHelmer *MockHelmer) {
	if name == "" {
		name = pegomock.AnyString()
	} else {
		name = pegomock.EqString(name)
	}
	if version == "" {
		version = pegomock.AnyString()
	} else {
		version = pegomock.EqString(version)
	}
	untar := pegomock.AnyBool()
	untardir := pegomock.AnyString()
	if repo == "" {
		repo = pegomock.AnyString()
	} else {
		repo = pegomock.EqString(repo)
	}

	pegomock.When(mockHelmer.FetchChart(
		name,
		version,
		untar,
		untardir,
		repo,
		pegomock.AnyString(),
		pegomock.AnyString())).
		Then(func(params []pegomock.Param) pegomock.ReturnValues {

			// We need to create the chart in dir
			fetchDir, err := util.AsString(params[3])
			if err != nil {
				return pegomock.ReturnValues{
					err,
				}
			}
			dir := filepath.Join(fetchDir, chartToCreate.Metadata.Name)
			err = os.MkdirAll(dir, 0700)
			if err != nil {
				return pegomock.ReturnValues{
					err,
				}
			}
			log.Logger().Infof("Creating mock chart %s in %s", chartToCreate.Metadata.Name, dir)
			err = helm.SaveFile(filepath.Join(dir, helm.ChartFileName), chartToCreate.Metadata)
			if err != nil {
				return pegomock.ReturnValues{
					err,
				}
			}
			err = helm.SaveFile(filepath.Join(dir, helm.RequirementsFileName), chartToCreate.Dependencies)
			if err != nil {
				return pegomock.ReturnValues{
					err,
				}
			}
			err = helm.SaveFile(filepath.Join(dir, helm.ValuesFileName), chartToCreate.Values)
			if err != nil {
				return pegomock.ReturnValues{
					err,
				}
			}
			err = os.MkdirAll(filepath.Join(dir, "templates"), 0700)
			if err != nil {
				return pegomock.ReturnValues{
					err,
				}
			}
			for _, t := range chartToCreate.Templates {
				err = ioutil.WriteFile(filepath.Join(dir, "templates", t.Name), t.Data, 0755)
				if err != nil {
					return pegomock.ReturnValues{
						err,
					}
				}
			}
			for _, f := range chartToCreate.Files {
				err = ioutil.WriteFile(filepath.Join(dir, f.TypeUrl), f.Value, 0755)
				if err != nil {
					return pegomock.ReturnValues{
						err,
					}
				}
			}
			return pegomock.ReturnValues{
				nil,
			}
		})
}
