package draft

import (
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	"github.com/Azure/draft/pkg/draft/draftpath"
	"github.com/Azure/draft/pkg/draft/pack/repo"

	"github.com/Azure/draft/pkg/linguist"
	"github.com/jenkins-x/jx/pkg/log"
)

// copied from draft so we can change the $DRAFT_HOME to ~/.jx/draft and lookup jx draft packs
// credit original from: https://github.com/Azure/draft/blob/8e1a459/cmd/draft/create.go#L163

// DoPackDetection performs pack detection across all the packs available in $(draft home)/packs in
// alphabetical order, returning the pack dirpath and any errors that occurred during the pack detection.
func DoPackDetection(home draftpath.Home, out io.Writer, dir string) (string, error) {
	log.Logger().Infof("performing pack detection in folder %s", dir)
	langs, err := linguist.ProcessDir(dir)
	if err != nil {
		return "", fmt.Errorf("there was an error detecting the language: %s", err)
	}
	if len(langs) == 0 {
		return "", fmt.Errorf("there was an error detecting the language")
	}
	for _, lang := range langs {
		detectedLang := linguist.Alias(lang)
		fmt.Fprintf(out, "--> Draft detected %s (%f%%)\n", detectedLang.Language, detectedLang.Percent)
		for _, repository := range repo.FindRepositories(home.Packs()) {
			packDir := path.Join(repository.Dir, repo.PackDirName)
			packs, err := ioutil.ReadDir(packDir)
			if err != nil {
				return "", fmt.Errorf("there was an error reading %s: %v", packDir, err)
			}
			for _, file := range packs {
				if file.IsDir() {
					if strings.Compare(strings.ToLower(detectedLang.Language), strings.ToLower(file.Name())) == 0 {
						packPath := filepath.Join(packDir, file.Name())
						return packPath, nil
					}
				}
			}
		}
		fmt.Fprintf(out, "--> Could not find a pack for %s. Trying to find the next likely language match...\n", detectedLang.Language)
	}
	return "", fmt.Errorf("there was an error detecting the language using packs from %s", home.Packs())
}

// DoPackDetectionForBuildPack performs detection of the language based on a sepcific build pack
func DoPackDetectionForBuildPack(out io.Writer, dir string, packDir string) (string, error) {
	log.Logger().Infof("performing pack detection in folder %s", dir)
	langs, err := linguist.ProcessDir(dir)
	if err != nil {
		return "", fmt.Errorf("there was an error detecting the language: %s", err)
	}
	if len(langs) == 0 {
		return "", fmt.Errorf("there was an error detecting the language")
	}
	for _, lang := range langs {
		detectedLang := linguist.Alias(lang)
		fmt.Fprintf(out, "--> Draft detected %s (%f%%)\n", detectedLang.Language, detectedLang.Percent)
		packs, err := ioutil.ReadDir(packDir)
		if err != nil {
			return "", fmt.Errorf("there was an error reading %s: %v", packDir, err)
		}
		for _, file := range packs {
			if file.IsDir() {
				if strings.Compare(strings.ToLower(detectedLang.Language), strings.ToLower(file.Name())) == 0 {
					packPath := filepath.Join(packDir, file.Name())
					return packPath, nil
				}
			}
		}
		fmt.Fprintf(out, "--> Could not find a pack for %s. Trying to find the next likely language match...\n", detectedLang.Language)
	}
	return "", fmt.Errorf("there was an error detecting the language using packs from %s", packDir)
}
