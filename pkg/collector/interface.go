package collector

// Collector an interface to collect data for storage in git or cloud storage etc
type Collector interface {

	// CollectFiles collects the given file paths and collects them into the storage
	// relative to the given output path. Returns the list of URLs to access the files
	CollectFiles(patterns []string, outputPath string, basedir string) ([]string, error)

	// CollectData collects the data storing it at the given output path and returning the URL
	// to access it
	CollectData(data []byte, outputPath string) (string, error)
}
