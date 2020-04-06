package iks

import (
	"fmt"
	"strings"

	"github.com/IBM-Cloud/bluemix-go/client"
)

type MachineType struct {
	Name                      string
	Memory                    string
	NetworkSpeed              string
	Cores                     string
	Os                        string
	ServerType                string
	Storage                   string
	SecondaryStorage          string
	SecondaryStorageEncrypted bool
	Deprecated                bool
	CorrespondingMachineType  string
	IsTrusted                 bool
	Gpus                      string
}

type VLAN struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Properties struct {
		Name                       string `json:"name"`
		Note                       string `json:"note"`
		PrimaryRouter              string `json:"primary_router"`
		VlanNumber                 string `json:"vlan_number"`
		VlanType                   string `json:"vlan_type"`
		Location                   string `json:"location"`
		LocalDiskStorageCapability string `json:"local_disk_storage_capability"`
		SanStorageCapability       string `json:"san_storage_capability"`
	} `json:"properties"`
}

type MachineTypes interface {
	GetMachineTypes(zone Zone, region Region) ([]MachineType, error)
	GetMachineType(machinetypearg string, zone Zone, region Region) (*MachineType, error)
}

type VLANs interface {
	GetVLANs(zone Zone, region Region) ([]VLAN, error)
	GetVLAN(vlanarg string, zone Zone, region Region) (*VLAN, error)
}

type machineTypes struct {
	*client.Client
	machineTypes map[string][]MachineType
}

type vLANs struct {
	*client.Client
	VLANs map[string][]VLAN
}

func newMachineTypesAPI(c *client.Client) MachineTypes {
	return &machineTypes{
		Client:       c,
		machineTypes: nil,
	}
}

func newVLANsAPI(c *client.Client) VLANs {
	return &vLANs{
		Client: c,
		VLANs:  nil,
	}
}

func (v *machineTypes) fetch(zone Zone, region Region) error {
	if v.machineTypes == nil {
		v.machineTypes = make(map[string][]MachineType)
	}
	if _, ok := v.machineTypes[zone.ID]; !ok {
		machineTypes := []MachineType{}
		headers := make(map[string]string, 2)
		headers["datacenter"] = zone.ID
		headers["X-Region"] = region.Name
		_, err := v.Client.Get("/v1/datacenters/"+zone.ID+"/machine-types", &machineTypes, headers)
		if err != nil {
			return err
		}
		v.machineTypes[zone.ID] = machineTypes
	}
	return nil
}

func (v *machineTypes) GetMachineTypes(zone Zone, region Region) ([]MachineType, error) {
	if err := v.fetch(zone, region); err != nil {
		return nil, err
	}
	return v.machineTypes[zone.ID], nil
}

func (v *machineTypes) GetMachineType(machinetypearg string, zone Zone, region Region) (*MachineType, error) {
	if err := v.fetch(zone, region); err != nil {
		return nil, err
	}

	for _, machineType := range v.machineTypes[zone.ID] {
		if strings.Compare(machinetypearg, machineType.Name) == 0 {
			return &machineType, nil
		}
	}
	return nil, fmt.Errorf("no machine type %q not found in zone %q", machinetypearg, zone.ID)
}

func (v *vLANs) fetch(zone Zone, region Region) error {
	if v.VLANs == nil {
		v.VLANs = make(map[string][]VLAN)
	}
	if _, ok := v.VLANs[zone.ID]; !ok {
		vLANs := []VLAN{}
		headers := make(map[string]string, 2)
		headers["datacenter"] = zone.ID
		headers["X-Region"] = region.Name
		_, err := v.Client.Get("/v1/datacenters/"+zone.ID+"/vlans", &vLANs, headers)
		if err != nil {
			return err
		}
		v.VLANs[zone.ID] = vLANs
	}
	return nil
}

func (v *vLANs) GetVLANs(zone Zone, region Region) ([]VLAN, error) {
	if err := v.fetch(zone, region); err != nil {
		return nil, err
	}
	return v.VLANs[zone.ID], nil
}

func (v *vLANs) GetVLAN(vlanarg string, zone Zone, region Region) (*VLAN, error) {
	if err := v.fetch(zone, region); err != nil {
		return nil, err
	}

	for _, vLAN := range v.VLANs[zone.ID] {
		if strings.Compare(vlanarg, vLAN.ID) == 0 {
			return &vLAN, nil
		}
	}
	return nil, fmt.Errorf("no machine type %q not found in zone %q", vlanarg, zone.ID)
}
