package server

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DedupeHTTPClient is a shared HTTP client for CDX requests
var DedupeHTTPClient = &http.Client{}

// checkCDX queries the CDX API to check for specific records using exact date and digest filtering
func checkCDX(CDXURL string, digest string, targetURI string, date string, cookie string) (bool, error) {
	// Format the date for from/to parameters (assuming date is in format YYYYMMDDHHMMSS)
	// Build URL with specific filters for exact matching
	requestURL := fmt.Sprintf("%s/web/timemap/cdx?url=%s&from=%s&to=%s&filter=digest:%s&limit=-1",
		CDXURL,
		url.QueryEscape(targetURI),
		date,
		date,
		digest,
	)

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return false, err
	}

	if cookie != "" {
		req.Header.Add("Cookie", cookie)
	}

	resp, err := DedupeHTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	// Split response into lines, then process each line
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// CDX format: ["urlkey","timestamp","original","mimetype","statuscode","digest","length"]
		cdxFields := strings.Fields(line)

		if len(cdxFields) >= 7 {
			// Fields: [0]urlkey [1]timestamp [2]original [3]mimetype [4]statuscode [5]digest [6]length
			recordDigest := cdxFields[5]

			// Since we already filtered by digest in the query, any result should match
			if recordDigest == digest && cdxFields[1] == date {
				return true, nil
			}
		}
	}

	// No matching record found
	return false, nil
}
