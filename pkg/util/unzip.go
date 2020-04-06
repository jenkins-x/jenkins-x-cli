package util

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Unzips the archvie into the specified directory
// returns an error if a general issue occurred unzipping the archive
func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		err = extractFile(dest, f)
		if err != nil {
			return err
		}
	}
	return nil
}

// Unzips the specified files from the archive
// returns an error if any of the specified files are not found or a general issue occurred unzipping the archive
func UnzipSpecificFiles(src, dest string, onlyFiles ...string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	m := make(map[string]bool)
	for _, f := range onlyFiles {
		m[f] = false
	}

	for _, f := range r.File {
		name := f.Name
		if _, matched := m[name]; matched {
			err = extractFile(dest, f)
			if err != nil {
				return err
			}
			m[name] = true
		}
	}

	// check we unzip all the specified files
	failed := false
	errString := ""
	for f, b := range m {
		if !b {
			if failed {
				errString += ", " + f
			} else {
				errString += ", " + f
				failed = true
			}
		}
	}
	if failed {
		return fmt.Errorf("the specified files where not found within the zip [%s]", errString)
	}

	return nil
}

// extract the specific file into the destination directory.
func extractFile(dest string, f *zip.File) error {
	name := filepath.Join(dest, f.Name) // #nosec
	// We need to be secure to prevent attacks like
	// https://snyk.io/blog/zip-slip-vulnerability
	// the result is already 'Clean'ed so we only need to check the string starts
	if !strings.HasPrefix(name, dest) {
		return fmt.Errorf("refusing to unzip %s due to escaping out of expected directory", f.Name)
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	if f.FileInfo().IsDir() {
		os.MkdirAll(name, os.ModePerm)
	} else {
		var fdir string
		if lastIndex := strings.LastIndex(name, string(os.PathSeparator)); lastIndex > -1 {
			fdir = name[:lastIndex]
		}

		err = os.MkdirAll(fdir, os.ModePerm)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(
			name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(f, rc)
		if err != nil {
			return err
		}
	}
	return nil
}
