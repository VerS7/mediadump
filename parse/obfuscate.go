package parse

import (
	"crypto/tls"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ObfuscatedClient struct {
	client     *http.Client
	userAgents []string
	mu         sync.RWMutex
	cookies    []*http.Cookie
}

type BrowserFingerprint struct {
	UserAgent          string
	Accept             string
	AcceptLanguage     string
	AcceptEncoding     string
	SecCHUA            string
	SecCHUAMobile      string
	SecCHUAArch        string
	SecCHUAPlatform    string
	SecCHUAModel       string
	SecCHUAFullVersion string
}

func NewObfuscatedClient() *ObfuscatedClient {
	jar, _ := cookiejar.New(nil)

	userAgents := []string{
		// Chrome Windows
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",

		// Chrome MacOS
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",

		// Firefox Windows
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/120.0",

		// Firefox MacOS
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/121.0",

		// Safari MacOS
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",

		// Edge Windows
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",

		// Mobile - iOS
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (iPad; CPU OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",

		// Mobile - Android
		"Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.230 Mobile Safari/537.36",
		"Mozilla/5.0 (Linux; Android 13; SM-G998B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.6045.163 Mobile Safari/537.36",
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				MaxVersion: tls.VersionTLS13,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				},
			},
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}

	return &ObfuscatedClient{
		client:     client,
		userAgents: userAgents,
		cookies:    make([]*http.Cookie, 0),
	}
}

func (c *ObfuscatedClient) GetRandomUserAgent() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userAgents[rand.Intn(len(c.userAgents))]
}

func (c *ObfuscatedClient) GetBrowserFingerprint() BrowserFingerprint {
	ua := c.GetRandomUserAgent()

	fingerprint := BrowserFingerprint{
		UserAgent:      ua,
		Accept:         "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		AcceptLanguage: "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7",
		AcceptEncoding: "gzip, deflate, br",
	}

	if strings.Contains(ua, "Chrome") && !strings.Contains(ua, "Edg") {
		fingerprint.SecCHUA = `"Google Chrome";v="120", "Chromium";v="120", "Not?A_Brand";v="8"`
		fingerprint.SecCHUAMobile = "?0"
		fingerprint.SecCHUAArch = "x86"
		fingerprint.SecCHUAPlatform = "Windows"
		fingerprint.SecCHUAModel = ""
		fingerprint.SecCHUAFullVersion = "120.0.6099.199"
	}

	if strings.Contains(ua, "Firefox") {
		fingerprint.SecCHUA = `"Not?A_Brand";v="8", "Chromium";v="120", "Firefox";v="121"`
		fingerprint.SecCHUAMobile = "?0"
	}

	if strings.Contains(ua, "Mobile") || strings.Contains(ua, "iPhone") || strings.Contains(ua, "Android") {
		fingerprint.SecCHUAMobile = "?1"
		fingerprint.Accept = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8"
	}

	return fingerprint
}

func (c *ObfuscatedClient) GetRequestHeaders(fingerprint BrowserFingerprint) map[string]string {
	headers := map[string]string{
		"User-Agent":                fingerprint.UserAgent,
		"Accept":                    fingerprint.Accept,
		"Accept-Language":           fingerprint.AcceptLanguage,
		"Accept-Encoding":           fingerprint.AcceptEncoding,
		"Connection":                "keep-alive",
		"Cache-Control":             "max-age=0",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
	}

	if fingerprint.SecCHUA != "" {
		headers["Sec-Ch-Ua"] = fingerprint.SecCHUA
		headers["Sec-Ch-Ua-Mobile"] = fingerprint.SecCHUAMobile
		headers["Sec-Ch-Ua-Platform"] = fingerprint.SecCHUAPlatform
	}

	if fingerprint.SecCHUAArch != "" {
		headers["Sec-Ch-Ua-Arch"] = fingerprint.SecCHUAArch
	}

	if fingerprint.SecCHUAModel != "" {
		headers["Sec-Ch-Ua-Model"] = fingerprint.SecCHUAModel
	}

	if rand.Float32() < 0.3 {
		headers["Dnt"] = "1"
	}

	if rand.Float32() < 0.2 {
		headers["Save-Data"] = "on"
	}

	return headers
}

func (c *ObfuscatedClient) MakeRequest(method, urlStr string) (*http.Response, error) {
	fingerprint := c.GetBrowserFingerprint()

	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		return nil, err
	}

	headers := c.GetRequestHeaders(fingerprint)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	c.randomDelay()

	return c.client.Do(req)
}

// MakeRequestWithBody выполняет POST/PUT запрос с телом
func (c *ObfuscatedClient) MakeRequestWithBody(method, urlStr string, body interface{}) (*http.Response, error) {
	// Здесь можно добавить преобразование body в io.Reader
	// Например, для JSON: json.Marshal(body)

	fingerprint := c.GetBrowserFingerprint()

	req, err := http.NewRequest(method, urlStr, nil) // Замените nil на body reader
	if err != nil {
		return nil, err
	}

	headers := c.GetRequestHeaders(fingerprint)

	// Для POST/PUT добавляем Content-Type
	if method == "POST" || method == "PUT" {
		contentTypes := []string{
			"application/json",
			"application/x-www-form-urlencoded",
			"multipart/form-data",
		}
		headers["Content-Type"] = contentTypes[rand.Intn(len(contentTypes))]
		headers["Content-Length"] = "0" // Реальный размер будет установлен автоматически
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	c.randomDelay()
	return c.client.Do(req)
}

func (c *ObfuscatedClient) randomDelay() {

	delay := time.Duration(100+rand.Intn(2900)) * time.Millisecond

	if rand.Float32() < 0.1 {
		delay = time.Duration(3+rand.Intn(10)) * time.Second
	}

	time.Sleep(delay)
}

func (c *ObfuscatedClient) SetCookies(u *url.URL, cookies []*http.Cookie) {
	c.client.Jar.SetCookies(u, cookies)
}

func (c *ObfuscatedClient) GetCookies(u *url.URL) []*http.Cookie {
	return c.client.Jar.Cookies(u)
}

func (c *ObfuscatedClient) Close() {
	c.client.CloseIdleConnections()
}
