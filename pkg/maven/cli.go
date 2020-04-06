package maven

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	filemutex "github.com/alexflint/go-filemutex"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/packages"
	"github.com/jenkins-x/jx/pkg/util"
)

// InstallMavenIfRequired installs maven if not available
func InstallMavenIfRequired() error {
	homeDir, err := util.ConfigDir()
	if err != nil {
		return err
	}
	m, err := filemutex.New(homeDir + "/jx.lock")
	if err != nil {
		panic(err)
	}
	m.Lock()

	cmd := util.Command{
		Name: "mvn",
		Args: []string{"-v"},
	}
	_, err = cmd.RunWithoutRetry()
	if err == nil {
		m.Unlock()
		return nil
	}
	// lets assume maven is not installed so lets download it
	clientURL := fmt.Sprintf("https://repo1.maven.org/maven2/org/apache/maven/apache-maven/%s/apache-maven-%s-bin.zip", MavenVersion, MavenVersion)

	log.Logger().Infof("Apache Maven is not installed so lets download: %s", util.ColorInfo(clientURL))

	mvnDir := filepath.Join(homeDir, "maven")
	mvnTmpDir := filepath.Join(homeDir, "maven-tmp")
	zipFile := filepath.Join(homeDir, "mvn.zip")

	err = os.MkdirAll(mvnDir, util.DefaultWritePermissions)
	if err != nil {
		m.Unlock()
		return err
	}

	log.Logger().Info("\ndownloadFile")
	err = packages.DownloadFile(clientURL, zipFile)
	if err != nil {
		m.Unlock()
		return err
	}

	log.Logger().Info("\nutil.Unzip")
	err = util.Unzip(zipFile, mvnTmpDir)
	if err != nil {
		m.Unlock()
		return err
	}

	// lets find a directory inside the unzipped folder
	log.Logger().Info("\nReadDir")
	files, err := ioutil.ReadDir(mvnTmpDir)
	if err != nil {
		m.Unlock()
		return err
	}
	for _, f := range files {
		name := f.Name()
		if f.IsDir() && strings.HasPrefix(name, "apache-maven") {
			os.RemoveAll(mvnDir)

			err = os.Rename(filepath.Join(mvnTmpDir, name), mvnDir)
			if err != nil {
				m.Unlock()
				return err
			}
			log.Logger().Infof("Apache Maven is installed at: %s", util.ColorInfo(mvnDir))
			m.Unlock()
			err = os.Remove(zipFile)
			if err != nil {
				m.Unlock()
				return err
			}
			err = os.RemoveAll(mvnTmpDir)
			if err != nil {
				m.Unlock()
				return err
			}
			m.Unlock()
			return nil
		}
	}
	m.Unlock()
	return fmt.Errorf("Could not find an apache-maven folder inside the unzipped maven distro at %s", mvnTmpDir)
}
