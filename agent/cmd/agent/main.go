package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

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

	if err := config.LoadEnvFile("agent/.env.local"); err != nil {
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

	preferencesStore, err := analyse.NewPreferencesStore("./agent/preferencesStore.db")
	if err != nil {
		return fmt.Errorf(
			"create preferencesStore Database: %w",
			err,
		)
	}
	defer preferencesStore.Close()

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

	toolsRegistry, err := tool.NewRegistry(
		readTextFileTool,
		listTextFilesTool,
		writeTextFileTool,
		searchTextTool,
	)

	if err != nil {
		return fmt.Errorf("工具注册表创建失败: %w", err)
	}

	runtime, err := agent.NewRuntime(client, cfg.Model.Name, cfg.Agent.SystemPrompt, toolsRegistry)
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
			if err := analyse.AnalyseProcedure(
				analyseRuntime,
				preferencesTransactionPlan,
				runtime.Messages,
			); err != nil {
				return err
			}
			log.Println("分析程序正常结束")
			return nil
		}

		reply, err := runtime.RunTurn(context.Background(), input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "本轮执行失败: %v\n", err)
			continue
		}

		fmt.Printf("AnXuan: %s\n", reply)
	}
}
