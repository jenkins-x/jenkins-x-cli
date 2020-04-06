package opts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cloud/openshift"
	"github.com/jenkins-x/jx/pkg/dependencymatrix"

	"github.com/jenkins-x/jx/pkg/virtualmachines/kvm"

	"github.com/jenkins-x/jx/pkg/virtualmachines/kvm2"

	"github.com/jenkins-x/jx/pkg/virtualmachines/virtualbox"

	"github.com/jenkins-x/jx/pkg/brew"

	"github.com/jenkins-x/jx/pkg/ksync"

	"github.com/jenkins-x/jx/pkg/cloud/amazon"

	"github.com/jenkins-x/jx/pkg/cloud/iks"

	randomdata "github.com/Pallinder/go-randomdata"
	"github.com/blang/semver"
	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/cloud/gke/externaldns"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/cluster"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/packages"
	"github.com/jenkins-x/jx/pkg/prow"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/pkg/errors"
	survey "gopkg.in/AlecAivazis/survey.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	groovy = `
// imports
import jenkins.model.Jenkins
import jenkins.model.JenkinsLocationConfiguration

// parameters
def jenkinsParameters = [
  url:    '%s/'
]

// get Jenkins location configuration
def jenkinsLocationConfiguration = JenkinsLocationConfiguration.get()

// set Jenkins URL
jenkinsLocationConfiguration.setUrl(jenkinsParameters.url)

// set Jenkins admin email address
jenkinsLocationConfiguration.setAdminAddress(jenkinsParameters.email)

// save current Jenkins state to disk
jenkinsLocationConfiguration.save()
`
)

const (
	AdminSecretsFile            = "adminSecrets.yaml"
	ExtraValuesFile             = "extraValues.yaml"
	ValuesFile                  = "values.yaml"
	JXInstallConfig             = "jx-install-config"
	CloudEnvValuesFile          = "myvalues.yaml"
	CloudEnvSecretsFile         = "secrets.yaml"
	CloudEnvSopsConfigFile      = ".sops.yaml"
	DefaultInstallTimeout       = "6000"
	DefaultCloudEnvironmentsURL = "https://github.com/jenkins-x/cloud-environments"
)

// DoInstallMissingDependencies install missing dependencies from the given list
func (o *CommonOptions) DoInstallMissingDependencies(install []string) error {
	// install package managers first
	for _, i := range install {
		if i == "brew" {
			log.Logger().Infof("Installing %s", util.ColorInfo(i))
			o.InstallBrew()
			break
		}
	}

	for _, i := range install {
		log.Logger().Infof("Installing %s", util.ColorInfo(i))
		var err error
		switch i {
		case "az":
			err = o.InstallAzureCli()
		case "kubectl":
			err = packages.InstallKubectl(false)
		case "gcloud":
			err = o.InstallGcloud()
		case "helm":
			err = o.InstallHelm()
		case "ibmcloud":
			err = iks.InstallIBMCloud(false)
		case "glooctl":
			err = o.InstallGlooctl()
		case "tiller":
			err = o.InstallTiller()
		case "helm3":
			err = o.InstallHelm3()
		case "kops":
			err = amazon.InstallKops()
		case "kvm":
			err = kvm.InstallKvm()
		case "kvm2":
			err = kvm2.InstallKvm2()
		case "ksync":
			_, err = ksync.InstallKSync()
		case "oc":
			err = openshift.InstallOc()
		case "virtualbox":
			err = virtualbox.InstallVirtualBox()
		case "xhyve":
			err = o.InstallXhyve()
		case "hyperv":
			err = o.Installhyperv()
		case "terraform":
			err = o.InstallTerraform()
		case "oci":
			err = o.InstallOciCli()
		case "aws":
			// Not yet implemented
		case "eksctl":
			err = amazon.InstallEksCtl(false)
		case "aws-iam-authenticator":
			err = amazon.InstallAwsIamAuthenticator(false)
		case "kustomize":
			err = o.InstallKustomize()
		default:
			return fmt.Errorf("unknown dependency to install %s\n", i)
		}
		if err != nil {
			return fmt.Errorf("error installing %s: %v\n", i, err)
		}
	}
	return nil
}

// InstallBrew installs brew
func (o *CommonOptions) InstallBrew() error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	log.Logger().Infof("Please enter your root password when prompted by the %s installation", util.ColorInfo("brew"))
	//Make sure to run command through sh in order to get $() expanded.
	return o.RunCommand("sh", "-c", "/usr/bin/ruby -e \"$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install)\"")
}

// InstallBrewIfRequired installs brew if required
func (o *CommonOptions) InstallBrewIfRequired() error {
	if runtime.GOOS != "darwin" || o.NoBrew {
		return nil
	}

	_, flag, err := packages.ShouldInstallBinary("brew")
	if err != nil || !flag {
		return err
	}
	return o.InstallBrew()
}

// InstallGlooctl Installs glooctl tool
func (o *CommonOptions) InstallGlooctl() error {
	if runtime.GOOS == "darwin" && !o.NoBrew {
		return o.RunCommand("brew", "install", "glooctl")
	}
	binDir, err := util.JXBinLocation()
	if err != nil {
		return err
	}
	fileName, flag, err := packages.ShouldInstallBinary("glooctl")
	if err != nil || !flag {
		return err
	}

	latestVersion, err := util.GetLatestVersionFromGitHub("solo-io", "gloo")
	if err != nil {
		return fmt.Errorf("unable to get latest version for github.com/%s/%s %v", "kubernetes-sigs", "kustomize", err)
	}

	suffix := runtime.GOARCH
	if runtime.GOOS == "windows" {
		suffix += ".exe"
	}
	clientURL := fmt.Sprintf("https://github.com/solo-io/gloo/releases/download/v%v/glooctl-%s-%s", latestVersion, runtime.GOOS, suffix)
	fullPath := filepath.Join(binDir, fileName)
	tmpFile := fullPath + ".tmp"
	err = packages.DownloadFile(clientURL, tmpFile)
	if err != nil {
		return err
	}
	err = util.RenameFile(tmpFile, fullPath)
	if err != nil {
		return err
	}
	return os.Chmod(fullPath, 0755)
}

// InstallKustomize installs kustomize
func (o *CommonOptions) InstallKustomize() error {
	if runtime.GOOS == "darwin" && !o.NoBrew {
		return o.RunCommand("brew", "install", "kustomize")
	}

	binDir, err := util.JXBinLocation()
	if err != nil {
		return errors.Wrapf(err, "unable to find JXBinLocation")
	}

	fullBinaryPath := filepath.Join(binDir, "kustomize")
	exists, err := util.FileExists(fullBinaryPath)
	if err != nil {
		return errors.Wrapf(err, "unable to verify if binary exists")
	}
	if exists {
		log.Logger().Debugf("binary %s already exists", fullBinaryPath)
		return nil
	}

	// get the stable jx supported version of kustomize to be install
	versionResolver, err := o.GetVersionResolver()
	if err != nil {
		return errors.Wrapf(err, "unable to retrieve version resolver")
	}

	stableVersion, err := versionResolver.StableVersion(versionstream.KindPackage, "kustomize")
	if err != nil {
		return errors.Wrapf(err, "unable to get stable version from the jenkins-x-versions for github.com/%s/%s %v ", "kubernetes-sigs", "kustomize", err)
	}

	clientURL := fmt.Sprintf("https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%%2Fv%s/kustomize_v%s_%s_%s.tar.gz", stableVersion.Version, stableVersion.Version, runtime.GOOS, runtime.GOARCH)
	tmpDir := filepath.Join(binDir, "kustomize.tmp")
	err = os.MkdirAll(tmpDir, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create tmp directory")
	}

	defer func() {
		err = os.RemoveAll(tmpDir)
		if err != nil {
			log.Logger().Warnf("Failed to Remove tmp directory: %v", err)
		}
	}()

	fullPath := filepath.Join(binDir, "kustomize")
	tarFile := filepath.Join(tmpDir, "kustomize.tar.gz")
	defer func() {
		err = os.Remove(tarFile)
		if err != nil {
			log.Logger().Warnf("failed to Remove tarFile : %v", err)
		}
	}()

	err = packages.DownloadFile(clientURL, tarFile)
	if err != nil {
		return errors.Wrapf(err, "failed to Download File")
	}
	err = util.UnTargz(tarFile, tmpDir, []string{"kustomize"})
	if err != nil {
		return errors.Wrapf(err, "failed to Un-tar file")
	}

	err = os.Rename(filepath.Join(tmpDir, "kustomize"), fullPath)
	if err != nil {
		return errors.Wrapf(err, "failed to rename file")
	}

	return os.Chmod(fullPath, 0755)
}

// InstallXhyve installs xhyve
func (o *CommonOptions) InstallXhyve() error {
	info, err := o.GetCommandOutput("", "brew", "info", "docker-machine-driver-xhyve")

	if err != nil || strings.Contains(info, "Not installed") {
		err = o.RunCommand("brew", "install", "docker-machine-driver-xhyve")
		if err != nil {
			return err
		}

		brewPrefix, err := o.GetCommandOutput("", "brew", "--prefix")
		if err != nil {
			return err
		}

		file := brewPrefix + "/opt/docker-machine-driver-xhyve/bin/docker-machine-driver-xhyve"
		err = o.RunCommand("sudo", "chown", "root:wheel", file)
		if err != nil {
			return err
		}

		err = o.RunCommand("sudo", "chmod", "u+s", file)
		if err != nil {
			return err
		}
		log.Logger().Info("xhyve driver installed")
	} else {
		pgmPath, _ := exec.LookPath("docker-machine-driver-xhyve")
		log.Logger().Infof("xhyve driver is already available on your PATH at %s", pgmPath)
	}
	return nil
}

// Installhyperv installs hyperv
func (o *CommonOptions) Installhyperv() error {
	info, err := o.GetCommandOutput("", "powershell", "Get-WindowsOptionalFeature", "-FeatureName", "Microsoft-Hyper-V-All", "-Online")

	if err != nil {
		return err
	}
	if strings.Contains(info, "Disabled") {

		log.Logger().Info("hyperv is Disabled, this computer will need to restart\n and after restart you will need to rerun your inputted command.")

		message := fmt.Sprintf("Would you like to restart your computer?")

		if answer, err := util.Confirm(message, true, "Please indicate if you would like to restart your computer.", o.GetIOFileHandles()); err != nil {
			return err
		} else if answer {

			err = o.RunCommand("powershell", "Enable-WindowsOptionalFeature", "-Online", "-FeatureName", "Microsoft-Hyper-V", "-All", "-NoRestart")
			if err != nil {
				return err
			}
			err = o.RunCommand("powershell", "Restart-Computer")
			if err != nil {
				return err
			}

		} else {
			err = errors.New("hyperv was not Disabled")
			return err
		}

	} else {
		log.Logger().Info("hyperv is already Enabled")
	}
	return nil
}

// InstallHelm install helm cli
func (o *CommonOptions) InstallHelm() error {
	binary := "helm"

	if runtime.GOOS == "darwin" && !o.NoBrew {
		err := o.RunCommand("brew", "install", "helm@2")
		if err != nil {
			return err
		}
		return o.installHelmSecretsPlugin(binary, true)
	}

	binDir, err := util.JXBinLocation()
	if err != nil {
		return err
	}

	fileName, flag, err := packages.ShouldInstallBinary(binary)
	if err != nil || !flag {
		return err
	}

	versionResolver, err := o.GetVersionResolver()
	if err != nil {
		return err
	}

	stableVersion, err := versionResolver.StableVersionNumber(versionstream.KindPackage, "helm")
	if err != nil {
		return err
	}

	clientURL := fmt.Sprintf("https://get.helm.sh/helm-v%s-%s-%s.tar.gz", stableVersion, runtime.GOOS, runtime.GOARCH)
	fullPath := filepath.Join(binDir, fileName)
	tarFile := fullPath + ".tgz"
	err = packages.DownloadFile(clientURL, tarFile)
	if err != nil {
		return err
	}
	err = util.UnTargz(tarFile, binDir, []string{binary, fileName})
	if err != nil {
		return err
	}
	err = os.Remove(tarFile)
	if err != nil {
		return err
	}
	err = os.Chmod(fullPath, 0755)
	if err != nil {
		return err
	}
	return o.installHelmSecretsPlugin(fullPath, true)
}

// InstallTiller installs tiller
func (o *CommonOptions) InstallTiller() error {
	binDir, err := util.JXBinLocation()
	if err != nil {
		return err
	}
	binary := "tiller"
	fileName := binary
	if runtime.GOOS == "windows" {
		fileName += ".exe"
	}
	// TODO workaround until 2.11.x GA is released
	latestVersion := "2.11.0-rc.3"
	/*
		latestVersion, err := util.GetLatestVersionFromGitHub("kubernetes", "helm")
			if err != nil {
				return err
			}
	*/
	clientURL := fmt.Sprintf("https://get.helm.sh/helm-v%s-%s-%s.tar.gz", latestVersion, runtime.GOOS, runtime.GOARCH)
	fullPath := filepath.Join(binDir, fileName)
	helmFullPath := filepath.Join(binDir, "helm")
	tarFile := fullPath + ".tgz"
	err = packages.DownloadFile(clientURL, tarFile)
	if err != nil {
		return err
	}
	err = util.UnTargz(tarFile, binDir, []string{binary, fileName, "helm"})
	if err != nil {
		return err
	}
	err = os.Remove(tarFile)
	if err != nil {
		return err
	}
	err = os.Chmod(fullPath, 0755)
	if err != nil {
		return err
	}
	err = helm.StartLocalTillerIfNotRunning()
	if err != nil {
		return err
	}
	return o.installHelmSecretsPlugin(helmFullPath, true)
}

// InstallHelm3 installs helm3 cli
func (o *CommonOptions) InstallHelm3() error {
	binDir, err := util.JXBinLocation()
	if err != nil {
		return err
	}
	binary := "helm3"
	fileName, flag, err := packages.ShouldInstallBinary(binary)
	if err != nil || !flag {
		return err
	}

	// https://get.helm.sh/helm-v3.0.0-alpha.1-darwin-amd64.tar.gz
	latestVersion := "v3.0.0-alpha.1"
	clientURL := fmt.Sprintf("https://get.helm.sh/helm-%v-%s-%s.tar.gz", latestVersion, runtime.GOOS, runtime.GOARCH)

	tmpDir := filepath.Join(binDir, "helm3.tmp")
	err = os.MkdirAll(tmpDir, util.DefaultWritePermissions)
	if err != nil {
		return err
	}
	fullPath := filepath.Join(binDir, binary)
	tarFile := filepath.Join(tmpDir, fileName+".tgz")
	err = packages.DownloadFile(clientURL, tarFile)
	if err != nil {
		return err
	}
	err = util.UnTargz(tarFile, tmpDir, []string{"helm", "helm"})
	if err != nil {
		return err
	}
	err = os.Remove(tarFile)
	if err != nil {
		return err
	}
	err = os.Rename(filepath.Join(tmpDir, "helm"), fullPath)
	if err != nil {
		return err
	}
	err = os.RemoveAll(tmpDir)
	if err != nil {
		return err
	}

	err = os.Chmod(fullPath, 0755)
	if err != nil {
		return err
	}
	return o.installHelmSecretsPlugin(fullPath, false)
}

func (o *CommonOptions) installHelmSecretsPlugin(helmBinary string, clientOnly bool) error {
	log.Logger().Infof("Installing %s", util.ColorInfo("helm secrets plugin"))
	err := o.Helm().Init(clientOnly, "", "", false)
	if err != nil {
		return errors.Wrap(err, "failed to initialize helm")
	}
	// remove the plugin just in case is already installed
	cmd := util.Command{
		Name: helmBinary,
		Args: []string{"plugin", "remove", "secrets"},
	}
	_, err = cmd.RunWithoutRetry()
	if err != nil && !strings.Contains(err.Error(), "secrets not found") {
		return errors.Wrap(err, "failed to remove helm secrets")
	}
	cmd = util.Command{
		Name: helmBinary,
		Args: []string{"plugin", "install", "https://github.com/futuresimple/helm-secrets"},
	}
	_, err = cmd.RunWithoutRetry()
	// Workaround for Helm install on Windows caused by https://github.com/helm/helm/issues/4418
	if err != nil && runtime.GOOS == "windows" && strings.Contains(err.Error(), "Error: symlink") {
		// The install _does_ seem to work, but we get an error - catch this on windows and lob it in the bin
		return nil
	}
	// End of Workaround
	return err
}

// InstallTerraform installs terraform
func (o *CommonOptions) InstallTerraform() error {
	if runtime.GOOS == "darwin" && !o.NoBrew {
		return o.RunCommand("brew", "install", "terraform")
	}

	binDir, err := util.JXBinLocation()
	if err != nil {
		return err
	}
	binary := "terraform"
	fileName, flag, err := packages.ShouldInstallBinary(binary)
	if err != nil || !flag {
		return err
	}
	latestVersion, err := util.GetLatestVersionFromGitHub("hashicorp", "terraform")
	if err != nil {
		return err
	}

	clientURL := fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_%s_%s.zip", latestVersion, latestVersion, runtime.GOOS, runtime.GOARCH)
	fullPath := filepath.Join(binDir, fileName)
	zipFile := fullPath + ".zip"
	err = packages.DownloadFile(clientURL, zipFile)
	if err != nil {
		return err
	}
	err = util.Unzip(zipFile, binDir)
	if err != nil {
		return err
	}
	err = os.Remove(zipFile)
	if err != nil {
		return err
	}
	return os.Chmod(fullPath, 0755)
}

// GetLatestJXVersion returns latest jx version
func (o *CommonOptions) GetLatestJXVersion(resolver *versionstream.VersionResolver) (semver.Version, error) {
	if config.LatestVersionStringsBucket != "" {
		err := o.InstallRequirements(cloud.GKE)
		if err != nil {
			return semver.Version{}, err
		}
		gcloudOpts := &gke.GCloud{}
		latestVersionStrings, err := gcloudOpts.ListObjects(config.LatestVersionStringsBucket, "binaries/jx")
		if err != nil {
			return semver.Version{}, nil
		}
		return util.GetLatestVersionStringFromBucketURLs(latestVersionStrings)
	}

	dir := resolver.VersionsDir
	matrix, err := dependencymatrix.LoadDependencyMatrix(dir)
	if err != nil {
		return semver.Version{}, errors.Wrapf(err, "failed to load dependency matrix from version stream at %s", dir)
	}
	for _, dep := range matrix.Dependencies {
		if dep.Host == "github.com" && dep.Owner == "jenkins-x" && dep.Repo == "jx" {
			v := dep.Version
			if v == "" {
				return semver.Version{}, fmt.Errorf("no version specified in the dependency matrix for version stream at %s", dir)
			}
			log.Logger().Debugf("found version %s of jx from the version stream", v)
			return semver.Make(v)
		}
	}
	log.Logger().Warnf("could not find the version of jx in the dependency matrix of the version stream at %s", dir)

	if runtime.GOOS == "darwin" && !o.NoBrew {
		log.Logger().Debugf("Locating latest JX version from HomeBrew")
		// incase auto-update is not enabled, lets perform an explicit brew update first
		brewUpdate, err := o.GetCommandOutput("", "brew", "update")
		if err != nil {
			log.Logger().Errorf("unable to update brew - %s", brewUpdate)
			return semver.Version{}, err
		}
		log.Logger().Debugf("updating brew - %s", brewUpdate)

		brewInfo, err := o.GetCommandOutput("", "brew", "info", "--json", "jx")
		if err != nil {
			log.Logger().Errorf("unable to get brew info for jx - %s", brewInfo)
			return semver.Version{}, err
		}

		v, err := brew.LatestJxBrewVersion(brewInfo)
		if err != nil {
			return semver.Version{}, err
		}

		return semver.Make(v)
	}
	log.Logger().Debugf("Locating latest JX version from GitHub")
	return util.GetLatestVersionFromGitHub("jenkins-x", "jx")
}

// InstallJx installs jx cli
func (o *CommonOptions) InstallJx(upgrade bool, version string) error {
	log.Logger().Debugf("installing jx %s", version)
	if runtime.GOOS == "darwin" && !o.NoBrew {
		if upgrade {
			return o.RunCommand("brew", "upgrade", "jx")
		} else {
			return o.RunCommand("brew", "install", "jx")
		}
	}
	binDir, err := util.JXBinLocation()
	if err != nil {
		return err
	}
	// Check for jx binary in non standard path and install there instead if found...
	nonStandardBinDir, err := util.JXBinaryLocation()
	if err == nil && binDir != nonStandardBinDir {
		binDir = nonStandardBinDir
	}
	binary := "jx"
	fileName := binary
	if !upgrade {
		f, flag, err := packages.ShouldInstallBinary(binary)
		if err != nil || !flag {
			return err
		}
		fileName = f
	}
	org := "jenkins-x"
	repo := "jx"
	if version == "" {
		latestVersion, err := util.GetLatestVersionFromGitHub(org, repo)
		if err != nil {
			return err
		}
		version = fmt.Sprintf("%s", latestVersion)
	}
	extension := "tar.gz"
	if runtime.GOOS == "windows" {
		extension = "zip"
	}
	clientURL := fmt.Sprintf("%s%s/"+binary+"-%s-%s.%s", config.BinaryDownloadBaseURL, version, runtime.GOOS, runtime.GOARCH, extension)
	fullPath := filepath.Join(binDir, fileName)
	if runtime.GOOS == "windows" {
		fullPath += ".exe"
	}
	tmpArchiveFile := fullPath + ".tmp"
	err = packages.DownloadFile(clientURL, tmpArchiveFile)
	if err != nil {
		return err
	}
	// Untar the new binary into a temp directory
	jxHome, err := util.ConfigDir()
	if err != nil {
		return err
	}

	if runtime.GOOS != "windows" {
		err = util.UnTargz(tmpArchiveFile, jxHome, []string{binary, fileName})
		if err != nil {
			return err
		}
		err = os.Remove(tmpArchiveFile)
		if err != nil {
			return err
		}
		err = os.Remove(filepath.Join(binDir, "jx"))
		if err != nil && o.Verbose {
			log.Logger().Infof("Skipping removal of old jx binary: %s", err)
		}
		// Copy over the new binary
		err = os.Rename(filepath.Join(jxHome, "jx"), filepath.Join(binDir, "jx"))
		if err != nil {
			return err
		}
	} else { // windows
		windowsBinaryFromArchive := "jx-windows-amd64.exe"
		err = util.UnzipSpecificFiles(tmpArchiveFile, jxHome, windowsBinaryFromArchive)
		if err != nil {
			return err
		}
		err = os.Remove(tmpArchiveFile)
		if err != nil {
			return err
		}
		// A standard remove and rename (or overwrite) will not work as the file will be locked as windows is running it
		// the trick is to rename to a tempfile :-o
		// this will leave old files around but well at least it updates.
		// we could schedule the file for cleanup at next boot but....
		// HKLM\System\CurrentControlSet\Control\Session Manager\PendingFileRenameOperations
		err = os.Rename(filepath.Join(binDir, "jx.exe"), filepath.Join(binDir, "jx.exe.deleteme"))
		// if we can not rename it this i pretty fatal as we won;t be able to overwrite either
		if err != nil {
			return err
		}
		// Copy over the new binary
		err = os.Rename(filepath.Join(jxHome, windowsBinaryFromArchive), filepath.Join(binDir, "jx.exe"))
		if err != nil {
			return err
		}
	}
	log.Logger().Infof("Jenkins X client has been installed into %s", util.ColorInfo(fullPath))
	return os.Chmod(fullPath, 0755)
}

// InstallGcloud installs gcloud cli
func (o *CommonOptions) InstallGcloud() error {
	if runtime.GOOS != "darwin" || o.NoBrew {
		return errors.New("please install missing gcloud sdk - see https://cloud.google.com/sdk/downloads#interactive")
	}
	return o.RunCommand("brew", "cask", "install", "google-cloud-sdk")
}

// InstallAzureCli installs azure cli
func (o *CommonOptions) InstallAzureCli() error {
	return o.RunCommand("brew", "install", "azure-cli")
}

// InstallOciCli installs oci cli
func (o *CommonOptions) InstallOciCli() error {
	var err error
	filePath := "./install.sh"
	log.Logger().Info("Installing OCI CLI...")
	err = o.RunCommand("curl", "-LO", "https://raw.githubusercontent.com/oracle/oci-cli/master/scripts/install/install.sh")

	if err != nil {
		return err
	}
	os.Chmod(filePath, 0755)

	err = o.RunCommandVerbose(filePath, "--accept-all-defaults")
	if err != nil {
		return err
	}

	return os.Remove(filePath)
}

// GetCloudProvider returns the cloud provider
func (o *CommonOptions) GetCloudProvider(p string) (string, error) {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	if p != "" {
		if !util.Contains(cloud.KubernetesProviders, p) {
			return "", util.InvalidArg(p, cloud.KubernetesProviders)
		}
	}

	if p == "" {
		prompt := &survey.Select{
			Message: "Cloud Provider",
			Options: cloud.KubernetesProviders,
			Help:    "Cloud service providing the Kubernetes cluster, Google (GKE), Oracle (OKE), Azure (AKS)",
		}

		survey.AskOne(prompt, &p, nil, surveyOpts)
	}
	return p, nil
}

// GetClusterDependencies returns the dependencies for a cloud provider
func (o *CommonOptions) GetClusterDependencies(depsToInstall []string) []string {
	deps := packages.FilterInstalledDependencies(depsToInstall)
	d := packages.BinaryShouldBeInstalled("kubectl")
	if d != "" && util.StringArrayIndex(deps, d) < 0 {
		deps = append(deps, d)
	}

	d = packages.BinaryShouldBeInstalled("helm")
	if d != "" && util.StringArrayIndex(deps, d) < 0 {
		deps = append(deps, d)
	}

	// Platform specific deps
	if runtime.GOOS == "darwin" {
		if !o.NoBrew {
			d = packages.BinaryShouldBeInstalled("brew")
			if d != "" && util.StringArrayIndex(deps, d) < 0 {
				deps = append(deps, d)
			}
		}
	}
	return deps
}

// InstallMissingDependencies installs missing dependencies
func (o *CommonOptions) InstallMissingDependencies(providerSpecificDeps []string) error {
	deps := o.GetClusterDependencies(providerSpecificDeps)
	if len(deps) == 0 {
		return nil
	}

	install := []string{}

	if o.InstallDependencies {
		install = append(install, deps...)
	} else {
		surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
		if o.BatchMode {
			return errors.New(fmt.Sprintf("run without batch mode or manually install missing dependencies %v\n", deps))
		}

		prompt := &survey.MultiSelect{
			Message: "Missing required dependencies, deselect to avoid auto installing:",
			Options: deps,
			Default: deps,
		}
		survey.AskOne(prompt, &install, nil, surveyOpts)
	}

	return o.DoInstallMissingDependencies(install)
}

// InstallRequirements installs any requirements for the given provider kind
func (o *CommonOptions) InstallRequirements(cloudProvider string, extraDependencies ...string) error {
	var deps []string
	switch cloudProvider {
	case cloud.IKS:
		deps = packages.AddRequiredBinary("ibmcloud", deps)
	case cloud.AWS:
		deps = packages.AddRequiredBinary("kops", deps)
	case cloud.EKS:
		deps = packages.AddRequiredBinary("eksctl", deps)
		deps = packages.AddRequiredBinary("aws-iam-authenticator", deps)
	case cloud.AKS:
		deps = packages.AddRequiredBinary("az", deps)
	case cloud.GKE:
		deps = packages.AddRequiredBinary("gcloud", deps)
	case cloud.OKE:
		deps = packages.AddRequiredBinary("oci", deps)
	}

	for _, dep := range extraDependencies {
		deps = packages.AddRequiredBinary(dep, deps)
	}

	return o.InstallMissingDependencies(deps)
}

// CreateClusterAdmin creates a cluster admin
func (o *CommonOptions) CreateClusterAdmin() error {

	content := []byte(
		`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: cluster-admin
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
- nonResourceURLs:
  - '*'
  verbs:
  - '*'`)

	fileName := randomdata.SillyName() + ".yml"
	fileName = filepath.Join(os.TempDir(), fileName)
	tmpfile, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}

	defer os.Remove(tmpfile.Name()) // clean up

	if _, err := tmpfile.Write(content); err != nil {
		return err
	}
	if err := tmpfile.Close(); err != nil {
		return err
	}

	_, err1 := o.GetCommandOutput("", "kubectl", "create", "clusterrolebinding", "kube-system-cluster-admin", "--clusterrole", "cluster-admin", "--serviceaccount", "kube-system:default")
	if err1 != nil {
		if strings.Contains(err1.Error(), "AlreadyExists") {
			log.Logger().Info("role cluster-admin already exists for the cluster")
		} else {
			return err1
		}
	}

	_, err2 := o.GetCommandOutput("", "kubectl", "create", "-f", tmpfile.Name())
	if err2 != nil {
		if strings.Contains(err2.Error(), "AlreadyExists") {
			log.Logger().Info("clusterroles.rbac.authorization.k8s.io 'cluster-admin' already exists")
		} else {
			return err2
		}
	}

	return nil
}

// GetClusterUserName returns cluster and user name
func (o *CommonOptions) GetClusterUserName() (string, error) {
	username, _ := o.GetCommandOutput("", "gcloud", "config", "get-value", "core/account")

	if username != "" {
		return cluster.GetSafeUsername(username), nil
	}

	config, _, err := o.Kube().LoadConfig()
	if err != nil {
		return username, errors.Wrap(err, "loading kube config")
	}
	if config == nil || config.Contexts == nil || len(config.Contexts) == 0 {
		return username, fmt.Errorf("No Kubernetes contexts available! Try create or connect to cluster?")
	}
	contextName := config.CurrentContext
	if contextName == "" {
		return username, fmt.Errorf("No Kubernetes context selected. Please select one (e.g. via jx context) first")
	}
	context := config.Contexts[contextName]
	if context == nil {
		return username, fmt.Errorf("No Kubernetes context available for context %s", contextName)
	}
	username = context.AuthInfo

	return username, nil
}

// InstallProw installs prow
func (o *CommonOptions) InstallProw(useTekton bool, useExternalDNS bool, isGitOps bool, gitOpsEnvDir string, gitUsername string, valuesFiles []string) error {
	if o.ReleaseName == "" {
		o.ReleaseName = kube.DefaultProwReleaseName
	}

	if o.Chart == "" {
		o.Chart = kube.ChartProw
	}

	var err error
	if o.HMACToken == "" {
		// why 41?  seems all examples so far have a random token of 41 chars
		o.HMACToken, err = util.RandStringBytesMaskImprSrc(41)
		if err != nil {
			return fmt.Errorf("cannot create a random hmac token for Prow")
		}
	}

	if o.OAUTHToken == "" {
		authConfigSvc, err := o.GitAuthConfigService()
		if err != nil {
			return errors.Wrap(err, "creating git auth config svc")
		}

		config := authConfigSvc.Config()
		// lets assume github.com for now so ignore config.CurrentServer
		server := config.GetOrCreateServer("https://github.com")
		message := fmt.Sprintf("%s bot user for CI/CD pipelines (not your personal Git user):", server.Label())
		userAuth, err := config.PickServerUserAuth(server, message, o.BatchMode, "", o.GetIOFileHandles())
		if err != nil {
			return errors.Wrap(err, "picking bot user auth")
		}
		o.OAUTHToken = userAuth.ApiToken
	}

	if o.Username == "" {
		o.Username, err = o.GetClusterUserName()
		if err != nil {
			return errors.Wrap(err, "retrieving the cluster user name")
		}
	}
	if gitUsername == "" {
		gitUsername = o.Username
	}

	client, devNamespace, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "creating kube client")
	}

	setValues := strings.Split(o.SetValues, ",")

	settings, err := o.TeamSettings()
	if err != nil {
		return errors.Wrap(err, "reading the team settings")
	}

	if !isGitOps {
		err = prow.AddDummyApplication(client, devNamespace, settings)
		if err != nil {
			return errors.Wrap(err, "adding dummy application")
		}
	}

	log.Logger().Infof("Installing Tekton into namespace %s", util.ColorInfo(devNamespace))

	ksecretValues := []string{}
	if settings.HelmTemplate || settings.NoTiller || settings.HelmBinary != "helm" {
		// lets disable tiller
		setValues = append(setValues, "tillerNamespace=")
	}

	prowVersion := o.Version

	setValues = append(setValues,
		"auth.git.username="+gitUsername)

	ksecretValues = append(ksecretValues,
		"auth.git.password="+o.OAUTHToken)

	err = o.Retry(2, time.Second, func() (err error) {
		return o.InstallChartOrGitOps(isGitOps, gitOpsEnvDir, kube.DefaultTektonReleaseName,
			kube.ChartTekton, "tekton", "", devNamespace, true, setValues, ksecretValues, valuesFiles, "")
	})
	if err != nil {
		return errors.Wrap(err, "failed to install Tekton")
	}

	setValues = append(setValues,
		"buildnum.enabled=false",
		"build.enabled=false",
		"pipelinerunner.enabled=true",
	)

	if useExternalDNS && strings.Contains(o.Domain, "nip.io") {
		log.Logger().Warnf("Skipping install of External DNS, %s domain is not supported while using External DNS", util.ColorInfo(o.Domain))
		log.Logger().Warnf("External DNS only supports the use of personally operated domains")
	} else if useExternalDNS && o.Domain != "" {
		log.Logger().Infof("Preparing to install ExternalDNS into namespace %s", util.ColorInfo(devNamespace))
		log.Logger().Infof("External DNS for Jenkins X is currently only supoorted on GKE")

		err = o.installExternalDNSGKE()
		if err != nil {
			return errors.Wrap(err, "failed to install external-dns")
		}
	}

	log.Logger().Infof("Installing Prow into namespace %s", util.ColorInfo(devNamespace))

	for _, value := range valuesFiles {
		log.Logger().Infof("with values file %s", util.ColorInfo(value))
	}

	secretValues := []string{"user=" + gitUsername, "oauthToken=" + o.OAUTHToken, "hmacToken=" + o.HMACToken}
	err = o.Retry(2, time.Second, func() (err error) {
		return o.InstallChartOrGitOps(isGitOps, gitOpsEnvDir, o.ReleaseName,
			o.Chart, "prow", prowVersion, devNamespace, true, setValues, secretValues, valuesFiles, "")
	})
	if err != nil {
		return errors.Wrap(err, "failed to install Prow")
	}

	if !useTekton {
		log.Logger().Infof("\nInstalling BuildTemplates into namespace %s", util.ColorInfo(devNamespace))
		err = o.Retry(2, time.Second, func() (err error) {
			return o.InstallChartOrGitOps(isGitOps, gitOpsEnvDir, kube.DefaultBuildTemplatesReleaseName,
				kube.ChartBuildTemplates, "jxbuildtemplates", "", devNamespace, true, nil, nil, nil, "")
		})
		if err != nil {
			return errors.Wrap(err, "failed to install JX Build Templates")
		}
	}
	return nil
}

// CreateWebhookProw create a webhook for prow using the given git provider
func (o *CommonOptions) CreateWebhookProw(gitURL string, gitProvider gits.GitProvider) error {
	client, err := o.KubeClient()
	if err != nil {
		return err
	}
	ns, _, err := kube.GetDevNamespace(client, o.currentNamespace)
	if err != nil {
		return err
	}
	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return err
	}
	baseURL, err := services.FindServiceURL(client, ns, "hook")
	if err != nil {
		return errors.Wrapf(err, "in namespace %s", ns)
	}
	if baseURL == "" {
		return fmt.Errorf("failed to find external URL of service hook in namespace %s", ns)
	}
	webhookUrl := util.UrlJoin(baseURL, "hook")

	hmacToken, err := client.CoreV1().Secrets(ns).Get("hmac-token", metav1.GetOptions{})
	if err != nil {
		return err
	}
	isInsecureSSL, err := o.IsInsecureSSLWebhooks()
	if err != nil {
		return errors.Wrapf(err, "failed to check if we need to setup insecure SSL webhook")
	}
	webhook := &gits.GitWebHookArguments{
		Owner:       gitInfo.Organisation,
		Repo:        gitInfo,
		URL:         webhookUrl,
		Secret:      string(hmacToken.Data["hmac"]),
		InsecureSSL: isInsecureSSL,
	}
	return gitProvider.CreateWebHook(webhook)
}

// IsProw checks if prow is available in the cluster
func (o *CommonOptions) IsProw() (bool, error) {
	ns := o.devNamespace
	jxClient, devNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return false, err
	}
	if ns == "" {
		ns = devNs
	}
	env, err := kube.GetEnvironment(jxClient, ns, "dev")
	if err != nil {
		return false, err
	}

	return env.Spec.TeamSettings.PromotionEngine == jenkinsv1.PromotionEngineProw, nil
}

func (o *CommonOptions) installExternalDNSGKE() error {

	if o.ReleaseName == "" {
		o.ReleaseName = kube.DefaultExternalDNSReleaseName
	}

	if o.Chart == "" {
		o.Chart = kube.ChartExternalDNS
	}

	var err error

	client, err := o.KubeClient()
	if err != nil {
		return err
	}

	devNamespace, _, err := kube.GetDevNamespace(client, o.currentNamespace)
	if err != nil {
		return fmt.Errorf("cannot find a dev team namespace to get existing exposecontroller config from. %v", err)
	}

	clusterName, err := cluster.Name(o.Kube())
	if err != nil {
		return errors.Wrap(err, "failed to get clusterName")
	}

	err = o.helm.AddRepo(kube.ChartOwnerExternalDNS, kube.ChartURLExternalDNS, "", "")
	if err != nil {
		return errors.Wrapf(err, "adding helm repo")
	}

	googleProjectID, err := gke.GetCurrentProject()
	if err != nil {
		return errors.Wrap(err, "failed to get project")
	}

	// Create managed zone for external dns if it doesn't exist
	var nameServers = []string{}
	gcloud := o.GCloud()
	err = gcloud.CreateManagedZone(googleProjectID, o.Domain)
	if err != nil {
		return errors.Wrap(err, "while trying to creating a CloudDNS managed zone for external-dns")
	}
	_, nameServers, err = gcloud.GetManagedZoneNameServers(googleProjectID, o.Domain)
	if err != nil {
		return errors.Wrap(err, "while trying to retrieve the managed zone name servers for external-dns")
	}

	o.NameServers = nameServers

	var gcpServiceAccountSecretName string
	gcpServiceAccountSecretName, err = externaldns.CreateExternalDNSGCPServiceAccount(o.GCloud(), client,
		kube.DefaultExternalDNSReleaseName, devNamespace, clusterName, googleProjectID)
	if err != nil {
		return errors.Wrap(err, "failed to create service account for ExternalDNS")
	}

	err = gcloud.EnableAPIs(googleProjectID, "dns")
	if err != nil {
		return errors.Wrap(err, "unable to enable 'dns' api")
	}

	sources := []string{
		"ingress",
	}

	sourcesList := "{" + strings.Join(sources, ", ") + "}"

	values := []string{
		"provider=" + "google",
		"sources=" + sourcesList,
		"rbac.create=" + "true",
		"google.serviceAccountSecret=" + gcpServiceAccountSecretName,
		"txt-owner-id=" + "jx-external-dns",
		"domainFilters=" + "{" + o.Domain + "}",
	}

	log.Logger().Infof("\nInstalling External DNS into namespace %s", util.ColorInfo(devNamespace))
	err = o.Retry(2, time.Second, func() (err error) {
		return o.InstallChartOrGitOps(false, "", kube.DefaultExternalDNSReleaseName, kube.ChartExternalDNS,
			kube.ChartExternalDNS, "", devNamespace, true, values, nil, nil, "")
	})
	if err != nil {
		return errors.Wrap(err, "failed to install External DNS")
	}

	return nil
}
