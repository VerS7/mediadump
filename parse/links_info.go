package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type OpenGraphData struct {
	Title       *string `json:"title,omitempty"`
	Type        *string `json:"type,omitempty"`
	Description *string `json:"description,omitempty"`
	URL         *string `json:"url,omitempty"`
	SiteName    *string `json:"siteName,omitempty"`
	Image       *string `json:"image,omitempty"`
}

type LinkEntry struct {
	Url    string         `json:"url"`
	Domain string         `json:"domain"`
	Date   string         `json:"date"`
	Unix   int64          `json:"unix"`
	OGData *OpenGraphData `json:"openGraph,omitempty"`
}

type DomainEntry struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

type ParsedLinksInfo struct {
	Count     int           `json:"count"`
	FirstDate string        `json:"firstDate"`
	LastDate  string        `json:"lastDate"`
	Domains   []DomainEntry `json:"domains"`
	Links     []LinkEntry   `json:"links"`
}

func ParseRawLine(raw string) (LinkEntry, error) {
	var lk LinkEntry
	var d string

	s := strings.Split(raw, " ")
	if len(s) != 4 {
		return lk, fmt.Errorf("wrong raw string: %s", raw)
	}

	ts, err := strconv.ParseInt(s[2], 10, 64)
	if err != nil {
		return lk, err
	}

	t := time.Unix(ts, 0)

	match := regexp.MustCompile(`https?://([a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`).FindStringSubmatch(s[3])

	switch len(match) {
	case 0:
		return lk, fmt.Errorf("wrong raw string: %s", raw)
	case 1:
		d = match[0]
	case 2:
		d = match[1]
	}

	return LinkEntry{
		Url:    s[3],
		Domain: d,
		Date:   t.String(),
		Unix:   t.Unix(),
		OGData: nil,
	}, nil
}
