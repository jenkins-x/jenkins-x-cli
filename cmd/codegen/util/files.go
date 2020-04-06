package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

// DeleteDirContents removes all the contents of the given directory
func DeleteDirContents(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return err
	}
	for _, file := range files {
		// lets ignore the top level dir
		if dir != file {
			err = os.RemoveAll(file)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// FileExists returns true if the specified path exist, false otherwise. An error is returned if and file system
// operation fails.
func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, errors.Wrapf(err, "failed to check if file exists %s", path)
}

// DeleteFile deletes a file from the operating system. This should NOT be used to delete any sensitive information
// because it can easily be recovered. Use DestroyFile to delete sensitive information
func DeleteFile(fileName string) (err error) {
	if fileName != "" {
		exists, err := FileExists(fileName)
		if err != nil {
			return fmt.Errorf("could not check if file exists %s due to %s", fileName, err)
		}

		if exists {
			err = os.Remove(fileName)
			if err != nil {
				return errors.Wrapf(err, "could not remove file due to %s", fileName)
			}
		}
	} else {
		return fmt.Errorf("filename is not valid")
	}
	return nil
}

// CopyFile copies a file from the specified source src to dst.
// credit https://gist.github.com/r0l1/92462b38df26839a3ca324697c8cba04
func CopyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return
	}

	err = out.Sync()
	if err != nil {
		return
	}

	si, err := os.Stat(src)
	if err != nil {
		return
	}
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return
	}

	return
}

// CopyDirPreserve copies from the src dir to the dst dir if the file does NOT already exist in dst
func CopyDirPreserve(src string, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		return errors.Wrapf(err, "checking %s exists", src)
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "checking %s exists", dst)
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return errors.Wrapf(err, "creating %s", dst)
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return errors.Wrapf(err, "reading files in %s", src)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDirPreserve(srcPath, dstPath)
			if err != nil {
				return errors.Wrapf(err, "recursively copying %s", entry.Name())
			}
		} else {
			// Skip symlinks.
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}
			if _, err := os.Stat(dstPath); os.IsNotExist(err) {
				err = CopyFile(srcPath, dstPath)
				if err != nil {
					return errors.Wrapf(err, "copying %s to %s", srcPath, dstPath)
				}
			} else if err != nil {
				return errors.Wrapf(err, "checking if %s exists", dstPath)
			}
		}
	}
	return nil
}

// DownloadFile downloads a file from the given URL
func DownloadFile(filepath string, url string) (err error) {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := GetClientWithTimeout(time.Hour * 2).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("download of %s failed with return code %d", url, resp.StatusCode)
		return err
	}

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	// make it executable
	os.Chmod(filepath, 0755)
	if err != nil {
		return err
	}
	return nil
}

// GetClientWithTimeout returns a client with JX default transport and user specified timeout
func GetClientWithTimeout(duration time.Duration) *http.Client {
	client := http.Client{}
	return &client
}
