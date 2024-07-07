package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	IP          string  `json:"ip"`
	Port        string  `json:"port"`
	Host        string  `json:"host"`
	CTLS        string  `json:"ctls,omitempty"`
	ZTLS        string  `json:"ztls,omitempty"`
	OpenSSL     string  `json:"openssl,omitempty"`
	Network     string  `json:"network,omitempty"`
	Version     string  `json:"version,omitempty"`
	City        string  `json:"city,omitempty"`
	Region      string  `json:"region,omitempty"`
	RegionCode  string  `json:"region_code,omitempty"`
	Country     string  `json:"country,omitempty"`
	CountryName string  `json:"country_name,omitempty"`
	CountryCode string  `json:"country_code,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
	Org         string  `json:"org,omitempty"`
}

func main() {
	outputFile := flag.String("o", "", "Output file")
	jsonFormat := flag.Bool("json", false, "Output in JSON format")
	debug := flag.Bool("debug", false, "Enable debug output")
	timeout := flag.Int("timeout", 5, "TLS connection timeout in seconds")
	retry := flag.Int("retry", 3, "Number of retries to perform for failures")
	flag.Parse()

	results := []Result{}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		target := scanner.Text()
		ip, port := parseTarget(target)
		host := retryGetHostFromIP(ip, port, *timeout, *retry)
		ctlsResult, ztlsResult, opensslResult := "", "", ""
		ipInfo := Result{}

		if *debug {
			ctlsResult = runCommand("ctls", ip+":"+port)
			ztlsResult = runCommand("ztls", ip+":"+port)
			opensslResult = runCommand("openssl", "s_client", "-connect", ip+":"+port)
			ipInfo = getIPInfo(ip)
		}

		result := Result{
			IP:          ip,
			Port:        port,
			Host:        host,
			CTLS:        ctlsResult,
			ZTLS:        ztlsResult,
			OpenSSL:     opensslResult,
			Network:     ipInfo.Network,
			Version:     ipInfo.Version,
			City:        ipInfo.City,
			Region:      ipInfo.Region,
			RegionCode:  ipInfo.RegionCode,
			Country:     ipInfo.Country,
			CountryName: ipInfo.CountryName,
			CountryCode: ipInfo.CountryCode,
			Latitude:    ipInfo.Latitude,
			Longitude:   ipInfo.Longitude,
			Timezone:    ipInfo.Timezone,
			Org:         ipInfo.Org,
		}
		results = append(results, result)

		if *jsonFormat {
			outputJSONLine(result, *outputFile)
		} else {
			outputPlainLine(result, *outputFile, *debug)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
		os.Exit(1)
	}

	if *outputFile != "" {
		if *jsonFormat {
			outputJSON(results, *outputFile)
		} else {
			outputPlain(results, *outputFile, *debug)
		}
	}
}

func parseTarget(target string) (string, string) {
	if strings.Contains(target, ":") {
		parts := strings.Split(target, ":")
		return parts[0], parts[1]
	}
	return target, "443"
}

func retryGetHostFromIP(ip, port string, timeout, retry int) string {
	var host string
	for i := 0; i < retry; i++ {
		host = getHostFromIP(ip, port, timeout)
		if host != "unknown" {
			return host
		}
	}
	return host
}

func getHostFromIP(ip, port string, timeout int) string {
	conn, err := tls.DialWithDialer(&net.Dialer{
		Timeout: time.Duration(timeout) * time.Second,
	}, "tcp", ip+":"+port, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return "unknown"
	}
	defer conn.Close()

	for _, cert := range conn.ConnectionState().PeerCertificates {
		for _, name := range cert.DNSNames {
			if strings.TrimSpace(name) != "" {
				return name
			}
		}
	}

	return "unknown"
}

func runCommand(command string, args ...string) string {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "error"
	}
	return strings.TrimSpace(string(output))
}

func getIPInfo(ip string) Result {
	resp, err := http.Get(fmt.Sprintf("https://ipapi.co/%s/json/", ip))
	if err != nil {
		return Result{}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Result{}
	}

	var result Result
	err = json.Unmarshal(body, &result)
	if err != nil {
		return Result{}
	}

	return result
}

func outputJSONLine(result Result, outputFile string) {
	jsonData, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if outputFile == "" {
		fmt.Println(string(jsonData))
	} else {
		f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		if _, err := f.WriteString(string(jsonData) + "\n"); err != nil {
			fmt.Fprintf(os.Stderr, "error writing to file: %v\n", err)
			os.Exit(1)
		}
	}
}

func outputPlainLine(result Result, outputFile string, debug bool) {
	var line string
	if debug {
		line = fmt.Sprintf("%s:%s [%s]\nCTLS: %s\nZTLS: %s\nOpenSSL: %s\nNetwork: %s\nVersion: %s\nCity: %s\nRegion: %s\nRegionCode: %s\nCountry: %s\nCountryName: %s\nCountryCode: %s\nLatitude: %f\nLongitude: %f\nTimezone: %s\nOrg: %s\n",
			result.IP, result.Port, result.Host, result.CTLS, result.ZTLS, result.OpenSSL, result.Network, result.Version, result.City, result.Region, result.RegionCode, result.Country, result.CountryName, result.CountryCode, result.Latitude, result.Longitude, result.Timezone, result.Org)
	} else {
		line = fmt.Sprintf("%s:%s [%s]\n", result.IP, result.Port, result.Host)
	}

	if outputFile == "" {
		fmt.Print(line)
	} else {
		f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		if _, err := f.WriteString(line); err != nil {
			fmt.Fprintf(os.Stderr, "error writing to file: %v\n", err)
			os.Exit(1)
		}
	}
}

func outputJSON(results []Result, outputFile string) {
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	err = os.WriteFile(outputFile, jsonData, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing to file: %v\n", err)
		os.Exit(1)
	}
}

func outputPlain(results []Result, outputFile string, debug bool) {
	var builder strings.Builder
	for _, result := range results {
		if debug {
			builder.WriteString(fmt.Sprintf("%s:%s [%s]\nCTLS: %s\nZTLS: %s\nOpenSSL: %s\nNetwork: %s\nVersion: %s\nCity: %s\nRegion: %s\nRegionCode: %s\nCountry: %s\nCountryName: %s\nCountryCode: %s\nLatitude: %f\nLongitude: %f\nTimezone: %s\nOrg: %s\n",
				result.IP, result.Port, result.Host, result.CTLS, result.ZTLS, result.OpenSSL, result.Network, result.Version, result.City, result.Region, result.RegionCode, result.Country, result.CountryName, result.CountryCode, result.Latitude, result.Longitude, result.Timezone, result.Org))
		} else {
			builder.WriteString(fmt.Sprintf("%s:%s [%s]\n", result.IP, result.Port, result.Host))
		}
	}

	err := os.WriteFile(outputFile, []byte(builder.String()), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing to file: %v\n", err)
		os.Exit(1)
	}
}
