package config

import (
	"reflect"
	"testing"
	"time"
)

var testTimeout1 = 10 * time.Second // 非缺少 REQUEST_TIMEOUT 时使用
var testTimeout2 = 60 * time.Second // 缺少 REQUEST_TIMEOUT 时使用

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name       string
		testEnvSet map[string]string
		wantErr    bool
		wantConfig Config
	}{
		{
			name: "正常配置和读取",
			testEnvSet: map[string]string{
				"AI_API_KEY":      "test-key",
				"MODEL":           defaultModelName,
				"LLM_API_URL":     "test-URL",
				"REQUEST_TIMEOUT": "10s",
				"SYSTEM_PROMPT":   "test-prompt",
			},
			wantErr: false,
			wantConfig: Config{
				Model: ModelConfig{
					APIKey:   "test-key",
					Name:     defaultModelName,
					Endpoint: "test-URL",
					Timeout:  testTimeout1,
				},
				Agent: AgentConfig{
					SystemPrompt: "test-prompt",
				},
			},
		},
		{
			name: "[环境配置值]存在空格 正常配置和读取",
			testEnvSet: map[string]string{
				"AI_API_KEY":      "  test-key ",
				"MODEL":           "  test-model  ",
				"LLM_API_URL":     "  test-URL  ",
				"REQUEST_TIMEOUT": "  10s  ",
				"SYSTEM_PROMPT":   "  test-prompt ",
			},
			wantErr: false,
			wantConfig: Config{
				Model: ModelConfig{
					APIKey:   "test-key",
					Name:     "test-model",
					Endpoint: "test-URL",
					Timeout:  testTimeout1,
				},
				Agent: AgentConfig{
					SystemPrompt: "test-prompt",
				},
			},
		},
		{
			name: "缺少 AI_API_KEY",
			testEnvSet: map[string]string{
				"MODEL":           defaultModelName,
				"LLM_API_URL":     "test-URL",
				"REQUEST_TIMEOUT": "10s",
				"SYSTEM_PROMPT":   "test-prompt",
			},
			wantErr: true,
		},
		{
			name: "缺少 MODEL",
			testEnvSet: map[string]string{
				"AI_API_KEY":      "test-key",
				"LLM_API_URL":     "test-URL",
				"REQUEST_TIMEOUT": "10s",
				"SYSTEM_PROMPT":   "test-prompt",
			},
			wantErr: false,
			wantConfig: Config{
				Model: ModelConfig{
					APIKey:   "test-key",
					Name:     defaultModelName,
					Endpoint: "test-URL",
					Timeout:  testTimeout1,
				},
				Agent: AgentConfig{
					SystemPrompt: "test-prompt",
				},
			},
		},
		{
			name: "缺少 LLM_API_URL",
			testEnvSet: map[string]string{
				"AI_API_KEY":      "test-key",
				"MODEL":           defaultModelName,
				"REQUEST_TIMEOUT": "10s",
				"SYSTEM_PROMPT":   "test-prompt",
			},
			wantErr: false,
			wantConfig: Config{
				Model: ModelConfig{
					APIKey:   "test-key",
					Name:     defaultModelName,
					Endpoint: "https://api.deepseek.com/v1/chat/completions",
					Timeout:  testTimeout1,
				},
				Agent: AgentConfig{
					SystemPrompt: "test-prompt",
				},
			},
		},
		{
			name: "缺少 REQUEST_TIMEOUT",
			testEnvSet: map[string]string{
				"AI_API_KEY":    "test-key",
				"MODEL":         defaultModelName,
				"LLM_API_URL":   "test-URL",
				"SYSTEM_PROMPT": "test-prompt",
			},
			wantErr: false,
			wantConfig: Config{
				Model: ModelConfig{
					APIKey:   "test-key",
					Name:     defaultModelName,
					Endpoint: "test-URL",
					Timeout:  testTimeout2,
				},
				Agent: AgentConfig{
					SystemPrompt: "test-prompt",
				},
			},
		},
		{
			name: "REQUEST_TIMEOUT 格式错误",
			testEnvSet: map[string]string{
				"AI_API_KEY":      "test-key",
				"MODEL":           defaultModelName,
				"LLM_API_URL":     "test-URL",
				"REQUEST_TIMEOUT": "invalid-duration",
				"SYSTEM_PROMPT":   "test-prompt",
			},
			wantErr: true,
		},
		{
			name: "缺少 SYSTEM_PROMPT",
			testEnvSet: map[string]string{
				"AI_API_KEY":      "test-key",
				"MODEL":           defaultModelName,
				"LLM_API_URL":     "test-URL",
				"REQUEST_TIMEOUT": "10s",
			},
			wantErr: false,
			wantConfig: Config{
				Model: ModelConfig{
					APIKey:   "test-key",
					Name:     defaultModelName,
					Endpoint: "test-URL",
					Timeout:  testTimeout1,
				},
				Agent: AgentConfig{
					SystemPrompt: defaultSystemPrompt,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{ // 将要测试的配置字段设置为"" 来保证后续测试基础环境干净
				"AI_API_KEY",
				"MODEL",
				"LLM_API_URL",
				"REQUEST_TIMEOUT",
				"SYSTEM_PROMPT",
			} {
				t.Setenv(key, "")
			}

			for k, v := range tt.testEnvSet {
				t.Setenv(k, v)
			}
			gotConfig, err := LoadConfig()

			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望出错但err为空, gotConfig: %v", gotConfig)
				}

				empty := Config{}
				if !reflect.DeepEqual(gotConfig, empty) {
					t.Fatalf("期望错误时返回空结构信息, gotConfig: %v", gotConfig)
				}

				return
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("期望正确但出错 gotConfig: %v, gotErr: %v", gotConfig, err)
			}

			if !reflect.DeepEqual(tt.wantConfig, gotConfig) {
				t.Fatalf("期望成功的测试的结果与期望的结构信息不符 wantConfig: %v, gotConfig: %v", tt.wantConfig, gotConfig)
			}
		})
	}
}
