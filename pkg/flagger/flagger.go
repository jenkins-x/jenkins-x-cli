package flagger

import (
	appsv1 "k8s.io/api/apps/v1"
)

// IsCanaryAuxiliaryDeployment returns whether this deployment has been created automatically by Flagger from a Canary object
func IsCanaryAuxiliaryDeployment(d appsv1.Deployment) bool {
	ownerReferences := d.GetObjectMeta().GetOwnerReferences()
	for i := range ownerReferences {
		if ownerReferences[i].Kind == "Canary" {
			return true
		}
	}
	return false
}
