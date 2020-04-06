package helm

// Version defines the helm version
type Version int

const (
	V2 Version = 2
	V3         = 3
)

type ChartSummary struct {
	Name         string
	ChartVersion string
	AppVersion   string
	Description  string
}

// ReleaseSummary is the information about a release in Helm
type ReleaseSummary struct {
	ReleaseName   string
	Revision      string
	Updated       string
	Status        string
	ChartFullName string
	Chart         string
	ChartVersion  string
	AppVersion    string
	Namespace     string
}
