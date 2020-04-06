package iks

import (
	"fmt"
	"strings"

	gohttp "net/http"

	ibmcloud "github.com/IBM-Cloud/bluemix-go"
	"github.com/IBM-Cloud/bluemix-go/client"
	"github.com/IBM-Cloud/bluemix-go/http"
	"github.com/IBM-Cloud/bluemix-go/rest"
)

const containerEndpointOfPublicBluemix = "https://containers.bluemix.net"

type Region struct {
	Name        string `json:"name"`
	Alias       string `json:"alias"`
	CfURL       string `json:"cfURL"`
	FreeEnabled bool   `json:"freeEnabled"`
}
type RegionResponse struct {
	Regions []Region `json:"regions"`
}

type Regions interface {
	GetRegions() ([]Region, error)
	GetRegion(region string) (*Region, error)
}

type region struct {
	*client.Client
	regions []Region
}

func newRegionsAPI(c *client.Client) Regions {
	return &region{
		Client:  c,
		regions: nil,
	}
}

func (v *region) fetch() error {
	if v.regions == nil {
		regions := RegionResponse{}
		_, err := v.Client.Get("/v1/regions", &regions)
		if err != nil {
			return err
		}
		v.regions = regions.Regions
	}
	return nil
}

//List ...
func (v *region) GetRegions() ([]Region, error) {
	if err := v.fetch(); err != nil {
		return nil, err
	}
	return v.regions, nil
}
func (v *region) GetRegion(regionarg string) (*Region, error) {
	if err := v.fetch(); err != nil {
		return nil, err
	}

	for _, region := range v.regions {
		if strings.Compare(regionarg, region.Name) == 0 {
			return &region, nil
		}
	}
	return nil, fmt.Errorf("region %q not found", regionarg)
}

func GetAuthRegions(config *ibmcloud.Config) ([]string, error) {

	regions := RegionResponse{}
	if config.HTTPClient == nil {
		config.HTTPClient = http.NewHTTPClient(config)
	}
	client := client.New(config, ibmcloud.MccpService, nil)
	resp, err := client.SendRequest(rest.GetRequest(containerEndpointOfPublicBluemix+"/v1/regions"), &regions)

	if resp.StatusCode == gohttp.StatusNotFound {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}

	strregions := make([]string, len(regions.Regions))

	for i, region := range regions.Regions {
		strregions[i] = region.Alias
	}

	return strregions, nil
}
