package parse

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/dyatlov/go-opengraph/opengraph"
)

type HttpParseJob struct {
	URL string
	ID  int
}

type HttpParseResult struct {
	Job     HttpParseJob
	Error   error
	Content string
	OGData  *OpenGraphData
}

type HttpParsePool struct {
	jobs    chan HttpParseJob
	results chan HttpParseResult

	wg sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc

	verbose bool
}

func NewHttpParsePool(verbose bool) *HttpParsePool {
	ctx, cancel := context.WithCancel(context.Background())

	return &HttpParsePool{
		jobs:    make(chan HttpParseJob),
		results: make(chan HttpParseResult),
		ctx:     ctx,
		cancel:  cancel,
		verbose: verbose,
	}
}

func (p *HttpParsePool) Swarm(count int) {
	for range count {
		p.Spawn()
	}

	go func() {
		p.wg.Wait()
		close(p.results)
	}()
}

func (p *HttpParsePool) Spawn() {
	p.wg.Add(1)
	go p.worker()
}

func (p *HttpParsePool) Wait() {
	p.wg.Wait()
}

func (p *HttpParsePool) Close() {
	p.cancel()
}

func (p *HttpParsePool) Feed(jobs []HttpParseJob) {
	go func() {
		defer close(p.jobs)

		for _, job := range jobs {
			select {
			case p.jobs <- job:
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

func (p *HttpParsePool) Results() <-chan HttpParseResult {
	return p.results
}

func (p *HttpParsePool) send(res HttpParseResult) bool {
	select {
	case p.results <- res:
		return true
	case <-p.ctx.Done():
		return false
	}
}

func (p *HttpParsePool) worker() {
	defer p.wg.Done()

	client := NewObfuscatedClient()

	for job := range p.jobs {
		if p.ctx.Err() != nil {
			return
		}

		if p.verbose {
			fmt.Printf("[JOB %d] Starting to process %s...\n", job.ID, job.URL)
		}

		resp, err := client.MakeRequest("GET", job.URL)
		if err != nil {
			if !p.send(HttpParseResult{Job: job, Error: err}) {
				return
			}
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err := fmt.Errorf(
				"url: %s responded with status code %d at job id: %d",
				job.URL, resp.StatusCode, job.ID,
			)

			if !p.send(HttpParseResult{Job: job, Error: err}) {
				return
			}
			continue
		}

		if readErr != nil {
			err := fmt.Errorf("url: %s at job id %d: reading body: %w", job.URL, job.ID, readErr)

			if !p.send(HttpParseResult{Job: job, Error: err}) {
				return
			}
			continue
		}

		body, err = decodeBody(resp, body)
		if err != nil {
			err := fmt.Errorf("url: %s at job id %d: %w", job.URL, job.ID, err)

			if !p.send(HttpParseResult{Job: job, Error: err}) {
				return
			}
			continue
		}

		og := opengraph.NewOpenGraph()
		if err := og.ProcessHTML(bytes.NewReader(body)); err != nil {
			if !p.send(HttpParseResult{Job: job, Error: err}) {
				return
			}
			continue
		}

		var image, title, ogType, desc, url, site string
		title = og.Title
		ogType = og.Type
		desc = og.Description
		url = og.URL
		site = og.SiteName
		if len(og.Images) != 0 {
			image = og.Images[0].URL
		}

		ogdata := OpenGraphData{
			Title:       &title,
			Type:        &ogType,
			Description: &desc,
			URL:         &url,
			SiteName:    &site,
			Image:       &image,
		}

		if p.verbose {
			fmt.Printf("[JOB %d] Done processing!\n", job.ID)
		}

		if !p.send(HttpParseResult{Job: job, Content: string(body), OGData: &ogdata}) {
			return
		}
	}
}

func decodeBody(resp *http.Response, body []byte) ([]byte, error) {
	if resp.Uncompressed || len(body) == 0 {
		return body, nil
	}

	var (
		reader io.Reader
		err    error
	)

	switch strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding"))) {
	case "", "identity":
		return body, nil
	case "gzip":
		reader, err = gzip.NewReader(bytes.NewReader(body))
	case "deflate":
		reader, err = zlib.NewReader(bytes.NewReader(body))
	case "br":
		reader = brotli.NewReader(bytes.NewReader(body))
	default:
		return body, nil
	}

	if err != nil {
		return nil, fmt.Errorf("decompressing %s body: %w", resp.Header.Get("Content-Encoding"), err)
	}

	decoded, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("decompressing %s body: %w", resp.Header.Get("Content-Encoding"), err)
	}

	return decoded, nil
}
