package collector

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

type storageInfo struct {
	StorageSummary struct {
		BinariesSummary struct {
			BinariesCount  string `json:"binariesCount"`
			BinariesSize   string `json:"binariesSize"`
			ArtifactsSize  string `json:"artifactsSize"`
			Optimization   string `json:"optimization"`
			ItemsCount     string `json:"itemsCount"`
			ArtifactsCount string `json:"artifactsCount"`
		} `json:"binariesSummary"`
		FileStoreSummary struct {
			StorageType      string `json:"storageType"`
			StorageDirectory string `json:"storageDirectory"`
			TotalSpace       string `json:"totalSpace"`
			UsedSpace        string `json:"usedSpace"`
			FreeSpace        string `json:"freeSpace"`
		} `json:"fileStoreSummary"`
		RepositoriesSummaryList []struct {
			RepoKey      string `json:"repoKey"`
			RepoType     string `json:"repoType"`
			FoldersCount int    `json:"foldersCount"`
			FilesCount   int    `json:"filesCount"`
			UsedSpace    string `json:"usedSpace"`
			ItemsCount   int    `json:"itemsCount"`
			PackageType  string `json:"packageType"`
			Percentage   string `json:"percentage"`
		} `json:"repositoriesSummaryList"`
	} `json:"storageSummary"`
	FileStoreSummary struct {
		StorageType      string `json:"storageType"`
		StorageDirectory string `json:"storageDirectory"`
		TotalSpace       string `json:"totalSpace"`
		UsedSpace        string `json:"usedSpace"`
		FreeSpace        string `json:"freeSpace"`
	} `json:"fileStoreSummary"`
	RepositoriesSummaryList []struct {
		RepoKey      string `json:"repoKey"`
		RepoType     string `json:"repoType"`
		FoldersCount int    `json:"foldersCount"`
		FilesCount   int    `json:"filesCount"`
		UsedSpace    string `json:"usedSpace"`
		ItemsCount   int    `json:"itemsCount"`
		PackageType  string `json:"packageType"`
		Percentage   string `json:"percentage"`
	} `json:"repositoriesSummaryList"`
	BinariesSummary struct {
		BinariesCount  string `json:"binariesCount"`
		BinariesSize   string `json:"binariesSize"`
		ArtifactsSize  string `json:"artifactsSize"`
		Optimization   string `json:"optimization"`
		ItemsCount     string `json:""`
		ArtifactsCount string `json:"artifactsCount"`
	} `json:"binariesSummary"`
}

func (e *Exporter) fetchStorageInfo() (storageInfo, error) {
	var storageInfo storageInfo
	resp, err := fetchHTTP(e.URI, "storageinfo", e.bc, e.sslVerify, e.timeout)
	if err != nil {
		return storageInfo, err
	}
	if err := json.Unmarshal(resp, &storageInfo); err != nil {
		e.jsonParseFailures.Inc()
		return storageInfo, err
	}
	return storageInfo, nil
}

func removeCommas(str string) (float64, error) {

	reg, err := regexp.Compile("[^0-9.]+")
	if err != nil {
		return 0, err
	}
	convertedStr, err := strconv.ParseFloat(reg.ReplaceAllString(str, ""), 64)
	if err != nil {
		return 0, err
	}

	return convertedStr, nil
}

func bytesConverter(str string) (float64, error) {
	type errorString struct {
		s string
	}
	num, err := removeCommas(str)
	if err != nil {
		return 0, err
	}

	if strings.Contains(str, "bytes") {
		return num, nil
	} else if strings.Contains(str, "KB") {
		return num * 1024, nil
	} else if strings.Contains(str, "MB") {
		return num * 1024 * 1024, nil
	} else if strings.Contains(str, "GB") {
		return num * 1024 * 1024 * 1024, nil
	} else if strings.Contains(str, "TB") {
		return num * 1024 * 1024 * 1024 * 1024, nil
	}
	return 0, fmt.Errorf("Could not convert %s to bytes", str)
}

func (e *Exporter) exportCount(metricName string, metric *prometheus.Desc, count string, ch chan<- prometheus.Metric) {
	if count == "" {
		e.jsonParseFailures.Inc()
		return
	}
	value, _ := removeCommas(count)
	ch <- prometheus.MustNewConstMetric(metric, prometheus.GaugeValue, value)
}

func (e *Exporter) exportSize(metricName string, metric *prometheus.Desc, size string, ch chan<- prometheus.Metric) {
	if size == "" {
		e.jsonParseFailures.Inc()
		return
	}
	value, _ := bytesConverter(size)
	ch <- prometheus.MustNewConstMetric(metric, prometheus.GaugeValue, value)
}

func (e *Exporter) exportFilestore(metricName string, metric *prometheus.Desc, size string, fileStoreType string, fileStoreDir string, ch chan<- prometheus.Metric) {
	if size == "" {
		e.jsonParseFailures.Inc()
		return
	}
	value, _ := bytesConverter(size)
	ch <- prometheus.MustNewConstMetric(metric, prometheus.GaugeValue, value, fileStoreType, fileStoreDir)
}

type repoSummary struct {
	Name         string
	Type         string
	FoldersCount float64
	FilesCount   float64
	UsedSpace    float64
	ItemsCount   float64
	PackageType  string
	Percentage   float64
}

func (e *Exporter) extractRepoSummary(storageInfo storageInfo, ch chan<- prometheus.Metric) {
	var err error
	rs := repoSummary{}
	repoSummaryList := []repoSummary{}
	for _, repo := range storageInfo.StorageSummary.RepositoriesSummaryList {
		if repo.RepoKey == "TOTAL" {
			continue
		}
		rs.Name = repo.RepoKey
		rs.Type = strings.ToLower(repo.RepoType)
		rs.FoldersCount = float64(repo.FoldersCount)
		rs.FilesCount = float64(repo.FilesCount)
		rs.ItemsCount = float64(repo.ItemsCount)
		rs.PackageType = strings.ToLower(repo.PackageType)
		rs.UsedSpace, err = bytesConverter(repo.UsedSpace)
		if err != nil {
			e.jsonParseFailures.Inc()
			return
		}
		rs.Percentage, err = removeCommas(repo.Percentage)
		if err != nil {
			e.jsonParseFailures.Inc()
			return
		}
		repoSummaryList = append(repoSummaryList, rs)
	}
	e.exportRepo(repoSummaryList, ch)
}

func (e *Exporter) exportRepo(repoSummaries []repoSummary, ch chan<- prometheus.Metric) {

	for _, repoSummary := range repoSummaries {
		for metricName, metric := range storageMetrics {
			switch metricName {
			case "repoUsed":
				ch <- prometheus.MustNewConstMetric(metric, prometheus.GaugeValue, repoSummary.UsedSpace, repoSummary.Name, repoSummary.Type, repoSummary.PackageType)
			case "repoFolders":
				ch <- prometheus.MustNewConstMetric(metric, prometheus.GaugeValue, repoSummary.FoldersCount, repoSummary.Name, repoSummary.Type, repoSummary.PackageType)
			case "repoItems":
				ch <- prometheus.MustNewConstMetric(metric, prometheus.GaugeValue, repoSummary.ItemsCount, repoSummary.Name, repoSummary.Type, repoSummary.PackageType)
			case "repoFiles":
				ch <- prometheus.MustNewConstMetric(metric, prometheus.GaugeValue, repoSummary.FilesCount, repoSummary.Name, repoSummary.Type, repoSummary.PackageType)
			case "repoPercentage":
				ch <- prometheus.MustNewConstMetric(metric, prometheus.GaugeValue, repoSummary.Percentage, repoSummary.Name, repoSummary.Type, repoSummary.PackageType)
			}
		}
	}
}
