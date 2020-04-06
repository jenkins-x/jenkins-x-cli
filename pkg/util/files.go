package util

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/log"
	"gopkg.in/AlecAivazis/survey.v1/terminal"

	"github.com/pkg/errors"
)

const (
	DefaultWritePermissions = 0760

	// DefaultFileWritePermissions default permissions when creating a file
	DefaultFileWritePermissions = 0644

	MaximumNewDirectoryAttempts = 1000
)

// IOFileHandles is a struct for holding CommonOptions' In, Out, and Err I/O handles, to simplify function calls.
type IOFileHandles struct {
	Err io.Writer
	In  terminal.FileReader
	Out terminal.FileWriter
}

// FileExists checks if path exists and is a file
func FileExists(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err == nil {
		return !fileInfo.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, errors.Wrapf(err, "failed to check if file exists %s", path)
}

// DirExists checks if path exists and is a directory
func DirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// FirstFileExists returns the first file which exists or an error if we can't detect if a file that exists
func FirstFileExists(paths ...string) (string, error) {
	for _, path := range paths {
		exists, err := FileExists(path)
		if err != nil {
			return "", err
		}
		if exists {
			return path, nil
		}
	}
	return "", nil
}

// FileIsEmpty checks if a file is empty
func FileIsEmpty(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return true, errors.Wrapf(err, "getting details of file '%s'", path)
	}
	return (fi.Size() == 0), nil
}

func IsEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}

// CreateUniqueDirectory creates a new directory but if the combination of dir and name exists
// then append a number until a unique name is found
func CreateUniqueDirectory(dir string, name string, maximumAttempts int) (string, error) {
	for i := 0; i < maximumAttempts; i++ {
		n := name
		if i > 0 {
			n += strconv.Itoa(i)
		}
		p := filepath.Join(dir, n)
		exists, err := FileExists(p)
		if err != nil {
			return p, err
		}
		if !exists {
			err := os.MkdirAll(p, DefaultWritePermissions)
			if err != nil {
				return "", fmt.Errorf("Failed to create directory %s due to %s", p, err)
			}
			return p, nil
		}
	}
	return "", fmt.Errorf("Could not create a unique file in %s starting with %s after %d attempts", dir, name, maximumAttempts)
}

func RenameDir(src string, dst string, force bool) (err error) {
	err = CopyDir(src, dst, force)
	if err != nil {
		return fmt.Errorf("failed to copy source dir %s to %s: %s", src, dst, err)
	}
	err = os.RemoveAll(src)
	if err != nil {
		return fmt.Errorf("failed to cleanup source dir %s: %s", src, err)
	}
	return nil
}

func RenameFile(src string, dst string) (err error) {
	if src == dst {
		return nil
	}
	err = CopyFile(src, dst)
	if err != nil {
		return fmt.Errorf("failed to copy source file %s to %s: %s", src, dst, err)
	}
	err = os.RemoveAll(src)
	if err != nil {
		return fmt.Errorf("failed to cleanup source file %s: %s", src, err)
	}
	return nil
}

// CopyFileOrDir copies the source file or directory to the given destination
func CopyFileOrDir(src string, dst string, force bool) (err error) {
	fi, err := os.Stat(src)
	if err != nil {
		return errors.Wrapf(err, "getting details of file '%s'", src)
	}
	if fi.IsDir() {
		return CopyDir(src, dst, force)
	}
	return CopyFile(src, dst)
}

// credit https://gist.github.com/r0l1/92462b38df26839a3ca324697c8cba04
func CopyDir(src string, dst string, force bool) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if err == nil {
		if force {
			os.RemoveAll(dst)
		} else {
			return fmt.Errorf("destination already exists")
		}
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath, force)
			if err != nil {
				return
			}
		} else {
			// Skip symlinks.
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}

			err = CopyFile(srcPath, dstPath)
			if err != nil {
				return
			}
		}
	}

	return
}

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

// CopyDirOverwrite copies from the source dir to the destination dir overwriting files along the way
func CopyDirOverwrite(src string, dst string) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDirOverwrite(srcPath, dstPath)
			if err != nil {
				return
			}
		} else {
			// Skip symlinks.
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}

			err = CopyFile(srcPath, dstPath)
			if err != nil {
				return
			}
		}
	}
	return
}

// loads a file
func LoadBytes(dir, name string) ([]byte, error) {
	path := filepath.Join(dir, name) // relative path
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error loading file %s in directory %s, %v", name, dir, err)
	}
	return bytes, nil
}

// DeleteFile deletes a file from the operating system. This should NOT be used to delete any sensitive information
// because it can easily be recovered. Use DestroyFile to delete sensitive information
func DeleteFile(fileName string) (err error) {
	if fileName != "" {
		exists, err := FileExists(fileName)
		if err != nil {
			return fmt.Errorf("Could not check if file exists %s due to %s", fileName, err)
		}

		if exists {
			err = os.Remove(fileName)
			if err != nil {
				return errors.Wrapf(err, "Could not remove file due to %s", fileName)
			}
		}
	} else {
		return fmt.Errorf("Filename is not valid")
	}
	return nil
}

// DestroyFile will securely delete a file by first overwriting it with random bytes, then deleting it. This should
// always be used for deleting sensitive information
func DestroyFile(filename string) error {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return errors.Wrapf(err, "Could not Destroy %s", filename)
	}
	size := fileInfo.Size()
	// Overwrite the file with random data. Doing this multiple times is probably more secure
	randomBytes := make([]byte, size)
	// Avoid false positive G404 of gosec module - https://github.com/securego/gosec/issues/291
	/* #nosec */
	_, _ = rand.Read(randomBytes)
	err = ioutil.WriteFile(filename, randomBytes, DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "Unable to overwrite %s with random data", filename)
	}
	return DeleteFile(filename)
}

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

func DeleteDirContentsExcept(dir string, exceptDir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return err
	}
	for _, file := range files {
		// lets ignore the top level dir
		if dir != file && !strings.HasSuffix(file, exceptDir) {
			err = os.RemoveAll(file)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteDirContents removes all the contents of the given directory
func RecreateDirs(dirs ...string) error {
	for _, dir := range dirs {
		err := os.RemoveAll(dir)
		if err != nil {
			return err
		}
		err = os.MkdirAll(dir, DefaultWritePermissions)
		if err != nil {
			return err
		}

	}
	return nil
}

// FilterFileExists filters out files which do not exist
func FilterFileExists(paths []string) []string {
	answer := []string{}
	for _, path := range paths {
		exists, err := FileExists(path)
		if exists && err == nil {
			answer = append(answer, path)
		}
	}
	return answer
}

// ContentTypeForFileName returns the MIME type for the given file name
func ContentTypeForFileName(name string) string {
	ext := filepath.Ext(name)
	answer := mime.TypeByExtension(ext)
	if answer == "" {
		if ext == ".log" || ext == ".txt" {
			return "text/plain; charset=utf-8"
		}
	}
	return answer
}

// IgnoreFile returns true if the path matches any of the ignores. The match is the same as filepath.Match.
func IgnoreFile(path string, ignores []string) (bool, error) {
	for _, ignore := range ignores {
		if matched, err := filepath.Match(ignore, path); err != nil {
			return false, errors.Wrapf(err, "error when matching ignore %s against path %s", ignore, path)
		} else if matched {
			return true, nil
		}
	}
	return false, nil
}

// ListDirectory logs the directory at path
func ListDirectory(root string, recurse bool) error {
	if info, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return errors.Wrapf(err, "unable to list %s as does not exist", root)
		}
		if !info.IsDir() {
			return errors.Errorf("%s is not a directory", root)
		}
		return errors.Wrapf(err, "stat %s", root)
	}
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		dir, _ := filepath.Split(path)
		if !recurse && dir != root {
			// No recursion and we aren't in the root dir
			return nil
		}
		info, err = os.Stat(path)
		if err != nil {
			return errors.Wrapf(err, "stat %s", path)
		}
		log.Logger().Infof("%v %d %s %s", info.Mode().String(), info.Size(), info.ModTime().Format(time.RFC822), info.Name())
		return nil
	})

}

// GlobAllFiles performs a glob on the pattern and then processes all the files found.
// if a folder matches the glob its treated as another glob to recurse into the directory
func GlobAllFiles(basedir string, pattern string, fn func(string) error) error {
	names, err := filepath.Glob(pattern)
	if err != nil {
		return errors.Wrapf(err, "failed to evaluate glob pattern '%s'", pattern)
	}
	for _, name := range names {
		fullPath := name
		if basedir != "" {
			fullPath = filepath.Join(basedir, name)
		}
		fi, err := os.Stat(fullPath)
		if err != nil {
			return errors.Wrapf(err, "getting details of file '%s'", fullPath)
		}
		if fi.IsDir() {
			err = GlobAllFiles("", filepath.Join(fullPath, "*"), fn)
			if err != nil {
				return err
			}
		} else {
			err = fn(fullPath)
			if err != nil {
				return errors.Wrapf(err, "failed processing file '%s'", fullPath)
			}
		}
	}
	return nil
}

// ToValidFileSystemName converts the name to one that can safely be used on the filesystem
func ToValidFileSystemName(name string) string {
	replacer := strings.NewReplacer(".", "_", "/", "_")
	return replacer.Replace(name)
}
