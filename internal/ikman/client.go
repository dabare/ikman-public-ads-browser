package ikman

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	BaseURL         string
	UserAgent       string
	RequestInterval time.Duration
}

type Client struct {
	baseURL   string
	userAgent string
	http      *http.Client
	cache     *cache
	limiter   *rateLimiter
}

func NewClient(cfg Config) *Client {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = publicHost
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "ikmanbrowser/0.1"
	}
	return &Client{
		baseURL:   baseURL,
		userAgent: cfg.UserAgent,
		http: &http.Client{
			Timeout: 20 * time.Second,
		},
		cache:   newCache(),
		limiter: newRateLimiter(cfg.RequestInterval),
	}
}

func (c *Client) Search(ctx context.Context, params SearchParams) (SearchResult, error) {
	path := params.ListingPath()
	sourceURL := c.baseURL + path
	html, err := c.fetch(ctx, sourceURL, 2*time.Minute)
	if err != nil {
		return SearchResult{SourceURL: sourceURL}, err
	}
	payload, err := ExtractInitialData(html)
	if err != nil {
		return SearchResult{SourceURL: sourceURL}, err
	}

	var data searchInitialData
	if err := json.Unmarshal(payload, &data); err != nil {
		return SearchResult{SourceURL: sourceURL}, fmt.Errorf("parse listing data: %w", err)
	}
	if data.Serp.Ads.Type != "Success" {
		c.cache.delete(sourceURL)
		return SearchResult{SourceURL: sourceURL}, fmt.Errorf("listing data unavailable: %s", data.Serp.Ads.Type)
	}

	result := SearchResult{
		Ads:       data.Serp.Ads.Data.Ads,
		Total:     firstPositive(data.Serp.Ads.Data.Total, data.Serp.Ads.Data.Count),
		SourceURL: sourceURL,
		Category:  data.Category,
	}
	if result.Total == 0 {
		result.Total = extractTotal(html)
	}
	if result.Total > 0 {
		result.TotalText = fmt.Sprintf("%s ads", comma(result.Total))
	}
	for i := range result.Ads {
		if phone := extractPublicPhone(result.Ads[i].Title + " " + result.Ads[i].Description + " " + result.Ads[i].Details); phone != "" {
			result.Ads[i].Phone = phone
			result.Ads[i].PhoneSource = "listing"
		}
	}
	return result, nil
}

func (c *Client) Detail(ctx context.Context, slug string) (Detail, error) {
	slug = cleanSlug(slug)
	if slug == "" {
		return Detail{}, errors.New("missing ad slug")
	}
	sourceURL := c.baseURL + "/en/ad/" + slug
	html, err := c.fetch(ctx, sourceURL, 15*time.Minute)
	if err != nil {
		return Detail{SourceURL: sourceURL}, err
	}
	payload, err := ExtractInitialData(html)
	if err != nil {
		return Detail{SourceURL: sourceURL}, err
	}

	var data detailInitialData
	if err := json.Unmarshal(payload, &data); err != nil {
		return Detail{SourceURL: sourceURL}, fmt.Errorf("parse detail data: %w", err)
	}
	if data.AdDetail.Type != "Success" {
		return Detail{SourceURL: sourceURL}, fmt.Errorf("detail data unavailable: %s", data.AdDetail.Type)
	}
	data.AdDetail.Data.SourceURL = sourceURL
	data.AdDetail.Data.Ad.SetGalleryURLs(extractDetailGalleryURLs(html, slug))
	return data.AdDetail.Data, nil
}

func (c *Client) EnrichPhones(ctx context.Context, ads []AdSummary) {
	type job struct {
		index int
		slug  string
	}
	jobs := make(chan job)
	var wg sync.WaitGroup

	for worker := 0; worker < 3; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				if ads[item.index].Phone != "" {
					continue
				}
				detail, err := c.Detail(ctx, item.slug)
				if err != nil {
					continue
				}
				EnrichSummaryFromDetail(&ads[item.index], detail)
			}
		}()
	}

	for i := range ads {
		if ads[i].Slug == "" {
			continue
		}
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		case jobs <- job{index: i, slug: ads[i].Slug}:
		}
	}
	close(jobs)
	wg.Wait()
}

func (c *Client) fetch(ctx context.Context, sourceURL string, ttl time.Duration) ([]byte, error) {
	if body, ok := c.cache.get(sourceURL); ok {
		return body, nil
	}
	c.limiter.wait()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("fetch %s: %s", sourceURL, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	c.cache.set(sourceURL, body, ttl)
	return body, nil
}

type searchInitialData struct {
	Category Category `json:"category"`
	Serp     struct {
		Ads struct {
			Type string `json:"type"`
			Data struct {
				Ads   []AdSummary `json:"ads"`
				Total int         `json:"total"`
				Count int         `json:"count"`
			} `json:"data"`
		} `json:"ads"`
	} `json:"serp"`
}

type detailInitialData struct {
	AdDetail struct {
		Type string `json:"type"`
		Data Detail `json:"data"`
	} `json:"adDetail"`
}

func extractTotal(html []byte) int {
	re := regexp.MustCompile(`Showing\s+[\d,]+-[\d,]+\s+of\s+([\d,]+)\s+ads`)
	matches := re.FindSubmatch(html)
	if len(matches) != 2 {
		return 0
	}
	value, _ := strconv.Atoi(strings.ReplaceAll(string(matches[1]), ",", ""))
	return value
}

func extractDetailGalleryURLs(html []byte, slug string) []string {
	if slug == "" {
		return nil
	}
	pattern := regexp.MustCompile(`https://i\.ikman-st\.com/` + regexp.QuoteMeta(slug) + `/([a-f0-9-]{36})(?:/[0-9]+/[0-9]+/(?:cropped|fitted)\.jpg)?`)
	matches := pattern.FindAllSubmatch(html, -1)
	seen := map[string]bool{}
	var urls []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		id := string(match[1])
		if seen[id] {
			continue
		}
		seen[id] = true
		urls = append(urls, imageURL("https://i.ikman-st.com", slug, id, 1200, 900, "fitted"))
	}
	return urls
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func comma(value int) string {
	raw := strconv.Itoa(value)
	if len(raw) <= 3 {
		return raw
	}
	var out []byte
	prefix := len(raw) % 3
	if prefix == 0 {
		prefix = 3
	}
	out = append(out, raw[:prefix]...)
	for i := prefix; i < len(raw); i += 3 {
		out = append(out, ',')
		out = append(out, raw[i:i+3]...)
	}
	return string(out)
}
