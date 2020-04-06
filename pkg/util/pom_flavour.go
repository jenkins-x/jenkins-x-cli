package util

import (
	"io/ioutil"
	"strings"
)

const (
	MAVEN        = "maven"
	MAVEN_JAVA11 = "maven-java11"
	APPSERVER    = "appserver"
	LIBERTY      = "liberty"
	DROPWIZARD   = "dropwizard"
)

func PomFlavour(path string) (string, error) {

	b, err := ioutil.ReadFile(path)
	if err != nil {
		return "", nil
	}

	s := string(b)
	if strings.Contains(s, "<packaging>war</packaging>") &&
		strings.Contains(s, "org.eclipse.microprofile") {
		return LIBERTY, nil
	}
	if strings.Contains(s, "<groupId>io.dropwizard") {
		return DROPWIZARD, nil
	}
	if strings.Contains(s, "<groupId>org.apache.tomcat") {
		return APPSERVER, nil
	}
	if strings.Contains(s, "<java.version>11</java.version>") {
		return MAVEN_JAVA11, nil
	}

	return MAVEN, nil
}
