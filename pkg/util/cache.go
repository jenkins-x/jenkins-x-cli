package util

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/jenkins-x/jx/pkg/log"
)

const (
	timeLayout                = time.RFC1123
	defaultFileWritePermisons = 0644

	// TODO make this configurable?
	defaultCacheTimeoutHours = 24
)

// CacheLoader defines cache value population callback that should be executed if cache entry with given key is
// not present.
type CacheLoader func() ([]byte, error)

// LoadCacheData loads cached data from the given cache file name and loader
func LoadCacheData(fileName string, loader CacheLoader) ([]byte, error) {
	if fileName == "" {
		return loader()
	}
	timecheckFileName := fileName + "_last_time_check"
	exists, _ := FileExists(fileName)
	if exists {
		// lets check if we should use cache
		if shouldUseCache(timecheckFileName) {
			return ioutil.ReadFile(fileName)
		}
	}
	data, err := loader()
	if err == nil {
		err2 := ioutil.WriteFile(fileName, data, defaultFileWritePermisons)
		if err2 != nil {
			log.Logger().Warnf("Failed to update cache file %s due to %s", fileName, err2)
		}
		writeTimeToFile(timecheckFileName, time.Now())
	}
	return data, err
}

// shouldUseCache returns true if we should use the cached data to serve up the content
func shouldUseCache(filePath string) bool {
	lastUpdateTime := getTimeFromFileIfExists(filePath)
	if time.Since(lastUpdateTime).Hours() < defaultCacheTimeoutHours {
		return true
	}
	return false
}

func writeTimeToFile(path string, inputTime time.Time) error {
	err := ioutil.WriteFile(path, []byte(inputTime.Format(timeLayout)), defaultFileWritePermisons)
	if err != nil {
		return fmt.Errorf("Error writing current update time to file: %s", err)
	}
	return nil
}

func getTimeFromFileIfExists(path string) time.Time {
	lastUpdateCheckTime, err := ioutil.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	timeInFile, err := time.Parse(timeLayout, string(lastUpdateCheckTime))
	if err != nil {
		return time.Time{}
	}
	return timeInFile
}
