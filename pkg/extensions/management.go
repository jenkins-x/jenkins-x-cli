package extensions

import (
	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/log"
)

// GetAndDeduplicateChildrenRecursively will walk a tree of extensions rooted at ext and add them to the flattened
// set exts. The lookupByUUID map is used to resolve UUID references to Extensions,
// and a warning will be emitted if the extension is not present in the lookupByUUID map.
func GetAndDeduplicateChildrenRecursively(ext jenkinsv1.Extension, lookupByUUID map[string]jenkinsv1.Extension,
	exts map[string]*jenkinsv1.Extension) error {
	// Add the ext
	exts[ext.Spec.UUID] = &ext
	// Recursively find exts
	for _, childUUID := range ext.Spec.Children {
		if child, ok := lookupByUUID[childUUID]; ok {
			err := GetAndDeduplicateChildrenRecursively(child, lookupByUUID, exts)
			if err != nil {
				return err
			}
		} else {
			log.Logger().Warnf("Unable to find child %s of %s", childUUID, ext.Spec.FullyQualifiedName())
		}
	}
	return nil
}
