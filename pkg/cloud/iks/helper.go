package iks

import (
	"strconv"
	"strings"

	"github.com/IBM-Cloud/bluemix-go/api/container/containerv1"
)

const unsupportedMinorKubeVersion = 11

func GetRegions(regions Regions) ([]string, error) {

	regionarr, err := regions.GetRegions()

	if err != nil {
		return nil, err
	}
	strregions := make([]string, len(regionarr))

	for i, region := range regionarr {
		strregions[i] = region.Name
	}

	return strregions, nil

}

func GetZones(region Region, zones Zones) ([]string, error) {
	zonearr, err := zones.GetZones(region)

	if err != nil {
		return nil, err
	}
	strzones := make([]string, len(zonearr))

	for i, zone := range zonearr {
		strzones[i] = zone.ID
	}

	return strzones, nil
}
func Filter(vs []containerv1.KubeVersion, f func(containerv1.KubeVersion) bool) []containerv1.KubeVersion {
	vsf := make([]containerv1.KubeVersion, 0)
	for _, v := range vs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

func isUnsupportedKubeVersion(version containerv1.KubeVersion) bool {
	return version.Minor < unsupportedMinorKubeVersion
}

func GetKubeVersions(versions containerv1.KubeVersions) ([]string, string, error) {
	target := containerv1.ClusterTargetHeader{}
	versionarr, err := versions.List(target)
	var def string

	if err != nil {
		return nil, "", err
	}

	// filter unsupported kube versions
	filteredVersionarr := Filter(versionarr, isUnsupportedKubeVersion)
	strversions := make([]string, len(filteredVersionarr))

	for i, version := range filteredVersionarr {
		strversions[i] = strconv.Itoa(version.Major) + "." + strconv.Itoa(version.Minor) + "." + strconv.Itoa(version.Patch)
		if version.Default {
			def = strversions[i]
		}
	}
	return strversions, def, nil
}

func GetMachineTypes(zone Zone, region Region, machinetypes MachineTypes) ([]string, error) {
	machinetypesarr, err := machinetypes.GetMachineTypes(zone, region)

	if err != nil {
		return nil, err
	}
	strmachinetype := make([]string, len(machinetypesarr))

	for i, machinetype := range machinetypesarr {
		strmachinetype[i] = machinetype.Name
	}

	return strmachinetype, nil
}

func GetPrivateVLANs(zone Zone, region Region, vlans VLANs) ([]string, error) {
	vlansarr, err := vlans.GetVLANs(zone, region)

	if err != nil {
		return nil, err
	}
	strvlans := make([]string, len(vlansarr))

	var i = 0
	for _, vlan := range vlansarr {
		if strings.Compare(vlan.Type, "private") == 0 {
			strvlans[i] = vlan.ID
			i++
		}
	}

	return strvlans[0:i], nil
}

func GetPublicVLANs(zone Zone, region Region, vlans VLANs) ([]string, error) {
	vlansarr, err := vlans.GetVLANs(zone, region)

	if err != nil {
		return nil, err
	}
	strvlans := make([]string, len(vlansarr))

	var i = 0
	for _, vlan := range vlansarr {
		if strings.Compare(vlan.Type, "public") == 0 {
			strvlans[i] = vlan.ID
			i++
		}
	}

	return strvlans[0:i], nil
}
