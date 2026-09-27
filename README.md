## 运行前准备：
1.本地创建.env.local文件，就是.env.example那个位置

## 须知:
当前各工具绑定"当前所在目录", 逻辑位于: Agent_AnXuan/agent/internal/workspace/workspace.go, 详情:"const DefaultDir = `.`".
正常来说没什么问题, 运行go run时cd到 '你的盘:\agent_an_xuan>' 就行.

## 运行：
终端输入: go run Agent_AnXuan/agent/cmd/agent/main.go

## Bro有话说：
个人练习项目，项目目标为"协作者"，

功能差的比较多，如常驻运行、网络搜索等常见能力，

第一版确认完成之前不写完整的README。

-狰和
2026/9/26