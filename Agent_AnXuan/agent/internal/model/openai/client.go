package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model"
)

// 客户端结构体
type Client struct {
	httpClient *http.Client
	endpoint   string
	apiKey     string
}

func NewClient(
	httpClient *http.Client,
	endpoint string,
	apiKey string,
) (*Client, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("httpClient 参数不能为 nil")
	}

	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("endpoint值不可为空")
	}

	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("apiKey值不可为空")
	}

	return &Client{
		httpClient: httpClient,
		endpoint:   strings.TrimSpace(endpoint),
		apiKey:     strings.TrimSpace(apiKey),
	}, nil
}

func (c *Client) Complete(ctx context.Context, request model.Request) (model.Response, error) {
	externalRequest, err := toChatCompletionRequest(request)
	if err != nil {
		return model.Response{}, err
	}

	requestJSON, err := json.Marshal(externalRequest)
	if err != nil {
		return model.Response{}, fmt.Errorf("外部请求编码失败, 请求: %v, 错误: %w", externalRequest, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(requestJSON))
	if err != nil {
		return model.Response{}, fmt.Errorf("请求创建失败, 错误: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return model.Response{}, fmt.Errorf("请求发送失败, 错误: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.Response{}, fmt.Errorf("读取响应体失败, 错误: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return model.Response{}, fmt.Errorf("响应码错误, 响应体: %s, 状态码: %d", string(respBytes), resp.StatusCode)
	}

	var chatCompletionResponse chatCompletionResponse
	if err := json.Unmarshal(respBytes, &chatCompletionResponse); err != nil {
		return model.Response{}, fmt.Errorf("响应解析失败, 错误: %w", err)
	}

	modelResponse, err := toModelResponse(chatCompletionResponse)
	if err != nil {
		return model.Response{}, err
	}

	return modelResponse, nil
}

var _ model.Model = (*Client)(nil)
