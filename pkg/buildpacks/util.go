package buildpacks

import (
	"fmt"

	"github.com/jenkins-x/draft-repo/pkg/draft/pack"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
)

// CopyBuildPack copies the build pack from the source dir to the destination dir
func CopyBuildPack(dest, src string) error {
	// first do some validation that we are copying from a valid pack directory
	p, err := pack.FromDir(src)
	if err != nil {
		return fmt.Errorf("could not load %s: %s", src, err)
	}

	// lets remove any files we think should be zapped
	for _, file := range []string{jenkinsfile.PipelineConfigFileName, jenkinsfile.PipelineTemplateFileName} {
		delete(p.Files, file)
	}
	return p.SaveDir(dest)
}
