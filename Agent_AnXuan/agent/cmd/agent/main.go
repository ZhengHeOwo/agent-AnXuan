package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/agent"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/analyse"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/config"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/model/openai"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/terminal"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/tool"
	"github.com/ZhengHeOwo/agent-AnXuan/agent/internal/workspace"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	console, err := terminal.NewConsole(os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("创建终端交互对象失败: %w", err)
	}

	if err := config.LoadEnvFile("local/config/.env.local"); err != nil {
		return fmt.Errorf("加载环境文件失败: %w", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("加载程序配置失败: %w", err)
	}

	httpClient := &http.Client{
		Timeout: cfg.Model.Timeout,
	}

	client, err := openai.NewClient(httpClient, cfg.Model.Endpoint, cfg.Model.APIKey)
	if err != nil {
		return fmt.Errorf("客户端配置失败: %w", err)
	}

	if err := os.MkdirAll(workspace.DefaultDir, 0o700); err != nil {
		return fmt.Errorf("create workspace directory %q: %w", workspace.DefaultDir, err)
	}

	projectWorkspace, err := workspace.OpenWorkspace(workspace.DefaultDir)
	if err != nil {
		return fmt.Errorf("open workspace %q: %w", workspace.DefaultDir, err)
	}
	defer func() {
		_ = projectWorkspace.Close()
	}()

	preferencesStore, err := analyse.NewPreferencesStore("./local/data/preferencesStore.db")
	if err != nil {
		return fmt.Errorf(
			"create preferencesStore Database: %w",
			err,
		)
	}
	defer preferencesStore.Close()

	retryAnalyseRuntime, retryPreferencesTransactionPlan, err := analyse.NewAnalyzeProgramConfiguration(
		client,
		cfg.Model.Name,
		preferencesStore,
	)
	if err != nil {
		return fmt.Errorf(
			"分析重试模型准备失败, 原因: %w",
			err,
		)
	}

	analyse.RetryAnalysis(retryAnalyseRuntime, retryPreferencesTransactionPlan, console)

	readTextFileTool, err := workspace.NewReadTextFileTool(projectWorkspace)
	if err != nil {
		return fmt.Errorf("create read_text_file tool: %w", err)
	}

	listTextFilesTool, err := workspace.NewListTextFilesTool(projectWorkspace)
	if err != nil {
		return fmt.Errorf("create list_text_files tool: %w", err)
	}

	writeTextFileTool, err := workspace.NewWriteTextFileTool(projectWorkspace, console)
	if err != nil {
		return fmt.Errorf("create write_text_file tool: %w", err)
	}

	searchTextTool, err := workspace.NewSearchTextTool(projectWorkspace)
	if err != nil {
		return fmt.Errorf("create search_text tool: %w", err)
	}

	toolsRegistry, err := tool.NewToolRegistry(
		readTextFileTool,
		listTextFilesTool,
		writeTextFileTool,
		searchTextTool,
	)

	if err != nil {
		return fmt.Errorf("工具注册表创建失败: %w", err)
	}

	mainModelPreference, err := preferencesStore.ListModelPreference()
	if err != nil {
		return fmt.Errorf(
			"get Main model preference failed: %w",
			err,
		)
	}

	analyseRuntime, preferencesTransactionPlan, err := analyse.NewAnalyzeProgramConfiguration(
		client,
		cfg.Model.Name,
		preferencesStore,
	)
	if err != nil {
		return fmt.Errorf(
			"The preparation for the analysis model has failed: %w",
			err,
		)
	}

	runtime, err := agent.NewRuntime(
		client,
		cfg.Model.Name,
		cfg.Agent.SystemPrompt,
		toolsRegistry,
		mainModelPreference,
	)
	if err != nil {
		return fmt.Errorf("创建Agent运行器失败: %w", err)
	}

	fmt.Println("Agent AnXuan 已启动, 输入 exit 退出")
	for {
		input, err := console.ReadLine(": ")
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return fmt.Errorf("读取终端失败: %w", err)
		}

		if input == "" {
			continue
		}

		if input == "exit" {
			ctx, cancel := context.WithCancel(context.Background())
			go ShowLoadingAnimation("分析程序", ctx)

			unanalyzedSampleError := analyse.AnalyseProcedure(
				analyseRuntime,
				preferencesTransactionPlan,
				runtime.Messages,
			)
			cancel()
			time.Sleep(500 * time.Millisecond)

			if unanalyzedSampleError != nil {
				summary := "未获取到样本详情"
				if unanalyzedSampleError.AnalysisSampleStats.Bytes != 0 || unanalyzedSampleError.AnalysisSampleStats.Turns != 0 {
					summary = fmt.Sprintf("当前样本\nBytes: %d | Turns: %d\n", unanalyzedSampleError.AnalysisSampleStats.Bytes, unanalyzedSampleError.AnalysisSampleStats.Turns)
				}

				confirmed, err := console.Confirm(
					context.Background(),
					tool.ConfirmationRequest{
						Action:  "是否保存未分析样本",
						Summary: summary,
						Details: fmt.Sprintf("错误来源:%q\n原始报错:%q\n", unanalyzedSampleError.DisplayErrorMessage.ErrorPosition, unanalyzedSampleError.DisplayErrorMessage.ErrorDetails),
					},
				)
				if err != nil {
					confirmed = true
					log.Println("询问失败, 已保存样本, 请及时处理!")
				}

				if confirmed {
					if err := preferencesStore.PutErrBucket(runtime.Messages); err != nil {
						log.Printf("存储失败 Owo 样本自由了!\n错误:%v", err)
						return nil
					}
				}
			}

			log.Println("分析程序结束")
			return nil
		}

		reply, err := runtime.RunTurn(context.Background(), input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "本轮执行失败: %v\n", err)
			continue
		}

		fmt.Printf("\n\n\nAnXuan: \n%s\n", reply)
	}
}

// ShowLoadingAnimation 加载动画.
func ShowLoadingAnimation(input string, ctx context.Context) {
	dots := []string{input + ".", input + "..", input + "..."}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Print("\r\033[K")
			return
		case <-ticker.C:
			fmt.Printf("\r\033[K%s", dots[i])
			i = (i + 1) % 3
		}
	}
}
