// Command igforge 是一个终端交互程序：按配置文件里定义的流水线，
// 自动完成"从 ModelScope 下载模型 -> （可选）量化 -> Docker 部署 vLLM"
// 这一整套操作。
//
// 用法：
//
//	go build -o igforge ./cmd/igforge
//	./igforge -config configs/models.json
//
// 后续规划（这一版暂不实现，但架构上已经留好扩展点）：
//   - k8s 部署：internal/pipeline/deploy.go 换成生成 Deployment/Service manifest
//     再 kubectl apply，下载和量化两个阶段完全不用改。
//   - 对接统一网关：部署成功后调用网关的注册 API，把新起的 vLLM 实例
//     （host、port、served-model-name）登记进网关的路由表。
//   - 可观测性：部署阶段顺带下发 Prometheus 的 scrape target（比如写一份
//     file_sd 格式的 json 文件到 Prometheus 会 watch 的目录），vLLM 自带
//     /metrics 端点，不需要额外 exporter。
//   - Vue3 管理控制台对接：把 pipeline 包整个通过一个小的 HTTP API 包一层
//     （复用 Runner 和 StageUpdate 的事件流，转成 SSE 推给前端），
//     控制台就能展示实时进度而不只是"跑完了才知道结果"。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"igforge/internal/config"
	"igforge/internal/tui"
)

func main() {
	configPath := flag.String("config", "configs/models.json", "流水线配置文件路径 (JSON)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(cfg.WorkDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "创建 work_dir 失败: %v\n", err)
		os.Exit(1)
	}

	// 捕获 Ctrl+C / SIGTERM，取消 context 让正在跑的子进程（下载/量化/docker）
	// 能收到中断信号退出，而不是留下孤儿进程继续占用 GPU 显存或网络带宽。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := tui.New(cfg)
	if err := app.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "运行出错: %v\n", err)
		os.Exit(1)
	}
}
