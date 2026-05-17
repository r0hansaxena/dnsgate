package blocker

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	c "github.com/ray-g/dnsproxy/cache"
	r "github.com/ray-g/dnsproxy/cache/record"
	conf "github.com/ray-g/dnsproxy/config"
	"github.com/ray-g/dnsproxy/logger"
	"github.com/ray-g/dnsproxy/stats"
	"github.com/ray-g/dnsproxy/utils"
)

var whitelist = make(map[string]bool)

func PerformUpdate(config *conf.BlockerConfig, cache c.Cache, forceUpdate bool) {
	populateWhitelist(config)
	loadStaticBlocklist(config, cache)

	if err := fetchSources(config.SourceURLs, config.SourceDir, forceUpdate); err != nil {
		logger.Fatal(err)
	}
	if err := loadSourcesIntoCache(cache, config.SourceDir); err != nil {
		logger.Fatal(err)
	}
}

func populateWhitelist(config *conf.BlockerConfig) {
	for _, entry := range config.Whitelist {
		whitelist[entry] = true
	}
}

func loadStaticBlocklist(config *conf.BlockerConfig, cache c.Cache) {
	for _, domain := range config.Blocklist {
		cache.Set(domain, r.NewBlockedRecord())
		stats.AddBlockedDomain()
	}
}

func fetchSources(sources []conf.DNSBlockSource, sourceDir string, force bool) error {
	var wg sync.WaitGroup

	for _, s := range sources {
		filename := fmt.Sprintf("%s.list", s.Name)
		_, err := os.Stat(filepath.Join(sourceDir, filename))
		if err == nil && !force {
			continue
		}

		wg.Add(1)
		go func(uri, name string) {
			defer wg.Done()
			logger.Debugf("fetching source %s", uri)
			if err := downloadFile(uri, name, sourceDir); err != nil {
				logger.Error("failed to download source, err: %v", err)
			}
		}(s.URL, filename)
	}

	wg.Wait()
	return nil
}

func downloadFile(uri, name, sourcedir string) error {
	utils.EnsureDirectory(sourcedir)
	filePath := filepath.FromSlash(filepath.Join(sourcedir, name))

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file: %s", err)
	}
	defer out.Close()

	resp, err := http.Get(uri)
	if err != nil {
		return fmt.Errorf("error downloading source: %s", err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("error copying output: %s", err)
	}
	return nil
}

func loadSourcesIntoCache(cache c.Cache, sourceDir string) error {
	logger.Debugf("loading blocked domains from %s ...", sourceDir)

	err := filepath.Walk(sourceDir, func(path string, f os.FileInfo, _ error) error {
		if !f.IsDir() {
			if err := parseHostFile(filepath.FromSlash(path), cache); err != nil {
				return fmt.Errorf("error parsing hostfile %s", err)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking location %s", err)
	}

	logger.Debugf("%d domains loaded from sources", cache.Length())
	return nil
}

func parseHostFile(fileName string, cache c.Cache) error {
	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("error opening file: %s", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.Split(line, "#")[0]
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) > 1 {
			line = fields[1]
		} else {
			line = fields[0]
		}

		if !cache.Exists(line) && !whitelist[line] {
			cache.Set(line, r.NewBlockedRecord())
			stats.AddBlockedDomain()
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning hostfile: %s", err)
	}
	return nil
}
