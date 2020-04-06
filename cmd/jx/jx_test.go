// +build unit

// +build covered_binary

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/pborman/uuid"

	"github.com/jenkins-x/jx/pkg/util"

	"github.com/stretchr/testify/assert"

	"github.com/jenkins-x/jx/cmd/jx/app"
)

func TestSystem(t *testing.T) {
	disableCoverage := os.Getenv("COVER_JX_BINARY") == "false"
	if !disableCoverage {
		disableSelfUpload := os.Getenv("COVER_SELF_UPLOAD") == "false"
		alreadyWrapped := os.Getenv("COVER_WRAPPED") == "true"
		coverprofileArgFound := false
		args := make([]string, 0)
		strippedArgs := make([]string, 0)
		for i, arg := range os.Args {
			if i > 0 && len(strippedArgs) > 0 && !strings.HasPrefix(arg, "-") && strings.HasPrefix(os.Args[i-1], "-") && os.Args[i-1] == strippedArgs[len(strippedArgs)-1] && !strings.Contains(os.Args[i-1], "=") {
				// This is an argument for the previous string
				strippedArgs = append(strippedArgs, arg)
			} else if !strings.HasPrefix(arg, "-test.") {
				args = append(args, arg)
			} else {
				strippedArgs = append(strippedArgs, arg)
			}
			if strings.HasPrefix(arg, "-test.coverprofile") {
				coverprofileArgFound = true
			}
		}
		if !alreadyWrapped && (!disableSelfUpload || !coverprofileArgFound) {
			// We need to wrap our own execution
			var outFile string
			var id string
			if !coverprofileArgFound {
				reportsDir := os.Getenv("REPORTS_DIR")
				if reportsDir == "" {
					reportsDir = filepath.Join("build", "reports")
				}
				err := os.MkdirAll(reportsDir, 0700)
				if err != nil {
					log.Logger().Errorf("Error making reports directory %s %v", reportsDir, err)
				}
				id = "jx"
				for _, arg := range args[1:] {
					if strings.HasPrefix(arg, "-") {
						break
					}
					id = fmt.Sprintf("%s_%s", id, arg)
				}
				args = append(args, "" /* use the zero value of the element type */)
				copy(args[2:], args[1:])
				outFile = filepath.Join(reportsDir, fmt.Sprintf("%s.%s.out", id, uuid.New()))
				args[1] = fmt.Sprintf("-test.coverprofile=%s", outFile)
			} else if !disableSelfUpload {
				// TODO support this
				log.Logger().Errorf("Self upload is not supported if -test.coverprofile is specified. Disabling it.")
				disableSelfUpload = true
			}
			cmd := util.Command{
				Env: map[string]string{
					"COVER_WRAPPED": "true",
				},
				Args: args[1:],
				Name: os.Args[0],
				Out:  os.Stdout,
				In:   os.Stdin,
				Err:  os.Stderr,
			}
			_, err := cmd.RunWithoutRetry()
			if err != nil {
				log.Logger().Error(err.Error())
				os.Exit(1)
			}
			os.Exit(0)
		} else {
			// Purposefully ignore errors from app.Run as we are checking coverage
			err := app.Run(args)
			// the assert.NoError defers the error reporting until after the coverage is written out
			assert.NoError(t, err, "error executing jx")
		}
	} else {
		main()
	}
}

func downloadFile(out *os.File, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
