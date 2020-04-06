package util

import (
	"bytes"
	"net/url"
	"strings"
)

// UrlJoin joins the given paths so that there is only ever one '/' character between the paths
func UrlJoin(paths ...string) string {
	var buffer bytes.Buffer
	last := len(paths) - 1
	for i, path := range paths {
		p := path
		if i > 0 {
			buffer.WriteString("/")
			p = strings.TrimPrefix(p, "/")
		}
		if i < last {
			p = strings.TrimSuffix(p, "/")
		}
		buffer.WriteString(p)
	}
	return buffer.String()
}

// UrlHostNameWithoutPort returns the host name without any port of the given URL like string
func UrlHostNameWithoutPort(rawUri string) (string, error) {
	if strings.Index(rawUri, ":/") > 0 {
		u, err := url.Parse(rawUri)
		if err != nil {
			return "", err
		}
		rawUri = u.Host
	}

	// must be a crazy kind of string so lets do our best
	slice := strings.Split(rawUri, ":")
	idx := 0
	if len(slice) > 1 {
		if len(slice) > 2 {
			idx = 1
		}
		return strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(slice[idx], "/"), "/"), "/"), nil
	}
	return rawUri, nil
}

// UrlEqual verifies if URLs are equal
func UrlEqual(url1, url2 string) bool {
	return url1 == url2 || strings.TrimSuffix(url1, "/") == strings.TrimSuffix(url2, "/")
}

// SanitizeURL sanitizes by stripping the user and password
func SanitizeURL(unsanitizedUrl string) string {
	u, err := url.Parse(unsanitizedUrl)
	if err != nil {
		return unsanitizedUrl
	}
	return stripCredentialsFromURL(u)
}

// stripCredentialsFromURL strip credentials from URL
func stripCredentialsFromURL(u *url.URL) string {
	pass, hasPassword := u.User.Password()
	userName := u.User.Username()
	if hasPassword {
		textToReplace := pass + "@"
		textToReplace = ":" + textToReplace
		if userName != "" {
			textToReplace = userName + textToReplace
		}
		return strings.Replace(u.String(), textToReplace, "", 1)
	}
	return u.String()
}

// URLToHostName converts the given URL to a host name returning the error string if its not a URL
func URLToHostName(svcURL string) string {
	host := ""
	if svcURL != "" {
		u, err := url.Parse(svcURL)
		if err != nil {
			host = err.Error()
		} else {
			host = u.Host
		}
	}
	return host
}
