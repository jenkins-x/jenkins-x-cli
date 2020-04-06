package prow

// Prow keeps install information for prow chart
type Prow struct {
	Version     string
	Chart       string
	SetValues   string
	ReleaseName string
	HMACToken   string
	OAUTHToken  string
}
