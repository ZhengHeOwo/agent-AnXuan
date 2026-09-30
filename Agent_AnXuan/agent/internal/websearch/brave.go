package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	braveEndpoint  = "https://api.search.brave.com/res/v1/web/search"
	requestTimeout = 15 * time.Second
	resultCount    = 8
)

type braveClient struct {
	apiKey     string
	httpClient *http.Client
}

func newBraveClient(apiKey string) *braveClient {
	return &braveClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

func (c *braveClient) search(ctx context.Context, query string) (*braveSearchResponse, error) {
	queryParams := url.Values{}
	queryParams.Set("q", query)
	queryParams.Set("count", strconv.Itoa(resultCount))
	queryParams.Set("extra_snippets", "true")
	queryParams.Set("text_decorations", "false")

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, braveEndpoint+"?"+queryParams.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("创建搜索请求失败, 错误: %w", err)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Subscription-Token", c.apiKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("发送搜索请求失败, 错误: %w", err)
	}
	defer response.Body.Close()

	responseBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("读取搜索响应失败, 错误: %w", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("响应码错误, 响应体: %s, 状态码: %d", string(responseBytes), response.StatusCode)
	}

	var searchResponse braveSearchResponse
	if err := json.Unmarshal(responseBytes, &searchResponse); err != nil {
		return nil, fmt.Errorf("响应解析失败, 错误: %w", err)
	}

	return &searchResponse, nil
}

type braveSearchResponse struct {
	Query braveQuery       `json:"query"`
	Web   *braveWebResults `json:"web"`
}

type braveQuery struct {
	Original string `json:"original"`
	Altered  string `json:"altered"`
}

type braveWebResults struct {
	Results []braveWebResult `json:"results"`
}

type braveWebResult struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	Description   string   `json:"description"`
	PageAge       string   `json:"page_age"`
	Age           string   `json:"age"`
	ExtraSnippets []string `json:"extra_snippets"`
}
