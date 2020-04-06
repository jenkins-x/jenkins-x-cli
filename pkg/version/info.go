// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package version

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/blang/semver"
	"github.com/jenkins-x/jx/pkg/log"
)

// Build information. Populated at build-time.
var (
	Version   string
	Revision  string
	Branch    string
	BuildUser string
	BuildDate string
	GoVersion string
)

// Map provides the iterable version information.
var Map = map[string]string{
	"version":   Version,
	"revision":  Revision,
	"branch":    Branch,
	"buildUser": BuildUser,
	"buildDate": BuildDate,
	"goVersion": GoVersion,
}

const (
	// VersionPrefix string for setting pre-release etc
	VersionPrefix = ""

	// ExampleVersion shows an example version in the help
	// if no version could be found (which should never really happen!)
	ExampleVersion = "1.1.59"

	// TestVersion used in test cases for the current version if no
	// version can be found - such as if the version property is not properly
	// included in the go test flags
	TestVersion = "2.0.404"
)

// GetVersion gets the current version string
func GetVersion() string {
	v := Map["version"]
	if v == "" {
		v = TestVersion
	}
	return v
}

// GetSemverVersion returns a semver.Version struct representing the current version
func GetSemverVersion() (semver.Version, error) {
	text := strings.TrimPrefix(GetVersion(), VersionPrefix)
	v, err := semver.Make(text)
	if err != nil {
		return v, errors.Wrapf(err, "failed to parse version %s", text)
	}
	return v, nil
}

// VersionStringDefault returns the current version string or returns a dummy
// default value if there is an error
func VersionStringDefault(defaultValue string) string {
	v, err := GetSemverVersion()
	if err == nil {
		return v.String()
	}
	log.Logger().Warnf("Warning failed to load version: %s", err)
	return defaultValue
}
