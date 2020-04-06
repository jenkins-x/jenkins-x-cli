package util

import (
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/cmd/codegen/util"
	"github.com/jenkins-x/jx/pkg/log"
)

func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h := os.Getenv("USERPROFILE") // windows
	if h == "" {
		h = "."
	}
	return h
}

// GitCredentialsFile returns the location of the git credentials file
func GitCredentialsFile() string {
	cfgHome := os.Getenv("XDG_CONFIG_HOME")
	if cfgHome == "" {
		cfgHome = util.HomeDir()
	}
	if cfgHome == "" {
		cfgHome = "."
	}
	return filepath.Join(cfgHome, "git", "credentials")
}

func DraftDir() (string, error) {
	c, err := ConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(c, "draft")
	err = os.MkdirAll(path, DefaultWritePermissions)
	if err != nil {
		return "", err
	}
	return path, nil
}

func ConfigDir() (string, error) {
	path := os.Getenv("JX_HOME")
	if path != "" {
		return path, nil
	}
	h := HomeDir()
	path = filepath.Join(h, ".jx")
	err := os.MkdirAll(path, DefaultWritePermissions)
	if err != nil {
		return "", err
	}
	return path, nil
}

// LocalFileSystemSecretsDir returns the default local file system secrets location for the file system alternative to vault
func LocalFileSystemSecretsDir() (string, error) {
	home, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "localSecrets"), nil
}

// KubeConfigFile gets the .kube/config file
func KubeConfigFile() string {
	path := os.Getenv("KUBECONFIG")
	if path != "" {
		return path
	}
	h := HomeDir()
	return filepath.Join(h, ".kube", "config")
}

// PluginBinDir returns the plugin bin directory for the given ns
func PluginBinDir(ns string) (string, error) {
	configDir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(configDir, "plugins", ns, "bin")
	err = os.MkdirAll(path, DefaultWritePermissions)
	if err != nil {
		return "", err
	}
	return path, nil
}

func CacheDir() (string, error) {
	h, err := ConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(h, "cache")
	err = os.MkdirAll(path, DefaultWritePermissions)
	if err != nil {
		return "", err
	}
	return path, nil
}

func EnvironmentsDir() (string, error) {
	h, err := ConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(h, "environments")
	err = os.MkdirAll(path, DefaultWritePermissions)
	if err != nil {
		return "", err
	}
	return path, nil
}

func OrganisationsDir() (string, error) {
	h, err := ConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(h, "organisations")
	err = os.MkdirAll(path, DefaultWritePermissions)
	if err != nil {
		return "", err
	}
	return path, nil
}

func BackupDir() (string, error) {
	h, err := ConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(h, "backup")
	err = os.MkdirAll(path, DefaultWritePermissions)
	if err != nil {
		return "", err
	}
	return path, nil
}

func LogsDir() (string, error) {
	h, err := ConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(h, "logs")
	err = os.MkdirAll(path, DefaultWritePermissions)
	if err != nil {
		return "", err
	}
	return path, nil
}

// JXBinLocation finds the JX config directory and creates a bin directory inside it if it does not already exist. Returns the JX bin path
func JXBinLocation() (string, error) {
	h, err := ConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(h, "bin")
	err = os.MkdirAll(path, DefaultWritePermissions)
	if err != nil {
		return "", err
	}
	return path, nil
}

// JXBinaryLocation Returns the path to the currently installed JX binary.
func JXBinaryLocation() (string, error) {
	return jXBinaryLocation(os.Executable)
}

func jXBinaryLocation(osExecutable func() (string, error)) (string, error) {
	jxProcessBinary, err := osExecutable()
	if err != nil {
		log.Logger().Debugf("jxProcessBinary error %s", err)
		return jxProcessBinary, err
	}
	log.Logger().Debugf("jxProcessBinary %s", jxProcessBinary)
	// make it absolute
	jxProcessBinary, err = filepath.Abs(jxProcessBinary)
	if err != nil {
		log.Logger().Debugf("jxProcessBinary error %s", err)
		return jxProcessBinary, err
	}
	log.Logger().Debugf("jxProcessBinary %s", jxProcessBinary)

	// if the process was started form a symlink go and get the absolute location.
	jxProcessBinary, err = filepath.EvalSymlinks(jxProcessBinary)
	if err != nil {
		log.Logger().Debugf("jxProcessBinary error %s", err)
		return jxProcessBinary, err
	}

	log.Logger().Debugf("jxProcessBinary %s", jxProcessBinary)
	path := filepath.Dir(jxProcessBinary)
	log.Logger().Debugf("dir from '%s' is '%s'", jxProcessBinary, path)
	return path, nil
}

func MavenBinaryLocation() (string, error) {
	h, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "maven", "bin"), nil
}
