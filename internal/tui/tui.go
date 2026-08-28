// Package tui 实现了一个不依赖任何第三方库的终端交互界面。
package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"igforge/internal/config"
	"igforge/internal/execx"
	"igforge/internal/pipeline"
)

// ANSI 颜色/样式常量。特意只用最基础、几乎所有终端模拟器（包括你常用的
// SSH 到阿里云 ECS 的场景）都支持的 8 色 + bold，不用 256 色或者 truecolor，
// 减少"本地好看、服务器上乱码"的风险。
const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	dim     = "\x1b[2m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	blue    = "\x1b[34m"
	magenta = "\x1b[35m"
	cyan    = "\x1b[36m"

	white = "\033[37m"
)

// App 持有运行整个交互流程所需的状态。
type App struct {
	Cfg    *config.Config
	Runner *pipeline.Runner
	in     *bufio.Reader
}

// New 构造一个 App。
func New(cfg *config.Config) *App {
	return &App{
		Cfg:    cfg,
		Runner: pipeline.NewRunner(cfg),
		in:     bufio.NewReader(os.Stdin),
	}
}

// Run 是主交互循环：展示模型列表 -> 用户选择 -> 展示流水线执行过程 -> 询问是否继续。
func (a *App) Run(ctx context.Context) error {
	a.banner()

	for {
		a.printModelList()
		fmt.Println()
		fmt.Print(dim + "输入模型编号执行流水线，输入 q 退出：" + reset + " ")

		line, err := a.readLine()
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "q" || line == "Q" {
			fmt.Println(dim + "再见 👋" + reset)
			return nil
		}

		idx, err := strconv.Atoi(line)
		if err != nil || idx < 1 || idx > len(a.Cfg.Models) {
			fmt.Println(red + "无效输入，请输入列表里的编号。" + reset)
			continue
		}

		m := a.Cfg.Models[idx-1]
		forceRedeploy := a.confirmForceRedeploy(m)

		fmt.Println()
		fmt.Println(bold + "── 开始执行流水线: " + m.ID + " ──" + reset)
		a.runPipeline(ctx, m, forceRedeploy)
		fmt.Println(bold + "── 流水线结束: " + m.ID + " ──" + reset)
		fmt.Println()
	}
}

func (a *App) banner() {
	logo := `
    ____  _ ____                    
   / __ \(_) __/___  _________ ____ 
  / /_/ / / /_/ __ \/ ___/ __ ` + "`" + `/ _ \
 / _, _/ / __/ /_/ / /  / /_/ /  __/
/_/ |_/_/_/  \____/_/   \__, /\___/ 
                       /____/       `

	fmt.Println(bold + cyan + logo + reset)
	fmt.Println()
	fmt.Println(bold + white + "  模型下载 / 量化 / 部署 自动化工具" + reset)
	fmt.Println(dim + "  ────────────────────────────────────" + reset)
	fmt.Println(dim + "  work_dir : " + reset + a.Cfg.WorkDir)
	//fmt.Println(dim + "  version  : " + reset + a.Cfg.Version) // 如果有版本号字段
	fmt.Println()
}

func (a *App) printModelList() {
	fmt.Println(bold + "可用流水线：" + reset)
	for i, m := range a.Cfg.Models {
		quantTag := dim + "无量化" + reset
		if m.Quantize.Enabled {
			quantTag = yellow + "量化:" + m.Quantize.Method + reset
		}

		downloaded := a.Runner.Downloader.AlreadyDownloaded(m)
		quantized := a.Runner.Quantizer.AlreadyQuantized(m)
		state, _ := a.Runner.Deployer.Inspect(context.Background(), m)

		fmt.Printf("  %s%2d)%s %-32s [%s] %s\n",
			bold, i+1, reset,
			m.ID,
			quantTag,
			fmt.Sprintf("端口 %d", m.Deploy.HostPort),
		)
		fmt.Printf("       %s下载:%s  %s量化:%s  %s部署:%s\n",
			dim, statusBadge(downloaded), dim, statusBadge(quantized), dim, deployBadge(state))
	}
}

func statusBadge(done bool) string {
	if done {
		return green + "✓ 已就绪" + reset
	}
	return dim + "○ 未完成" + reset
}

func deployBadge(state pipeline.ContainerState) string {
	switch state {
	case pipeline.ContainerRunning:
		return green + "✓ 运行中" + reset
	case pipeline.ContainerStoppedButExists:
		return yellow + "● 已停止" + reset
	default:
		return dim + "○ 未部署" + reset
	}
}

func (a *App) confirmForceRedeploy(m config.ModelEntry) bool {
	state, _ := a.Runner.Deployer.Inspect(context.Background(), m)
	if state != pipeline.ContainerRunning {
		return false // 没在跑，不存在"要不要强制重建"的问题
	}
	fmt.Printf(yellow+"容器 %s 已在运行中。是否停止并重新部署？[y/N] "+reset, m.Deploy.ContainerName)
	line, _ := a.readLine()
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}

// runPipeline 跑一次完整流水线，并把 Runner 推送的事件实时渲染出来。
func (a *App) runPipeline(ctx context.Context, m config.ModelEntry, forceRedeploy bool) {
	updates := make(chan pipeline.StageUpdate)
	go a.Runner.RunAll(ctx, m, forceRedeploy, updates)

	for u := range updates {
		switch {
		case u.Result != nil:
			printStageResult(*u.Result)
		case u.Log != nil:
			printLogLine(*u.Log)
		}
	}
}

func printLogLine(l execx.Line) {
	prefix := dim + "  │ " + reset
	clearLine := "\r\x1b[K"

	text := l.Text
	if l.Stderr {
		text = "\x1b[38;5;131m" + text + reset // 柔和的暗橙红
	}

	if l.Progress {
		fmt.Print(clearLine + prefix + text) // 不换行，等下一次更新原地覆盖
		return
	}
	fmt.Println(clearLine + prefix + text)
}

func printStageResult(r pipeline.StageResult) {
	var color, icon string
	switch r.Status {
	case pipeline.StatusRunning:
		color, icon = blue, "▶"
	case pipeline.StatusSkipped:
		color, icon = yellow, "⏭"
	case pipeline.StatusSuccess:
		color, icon = green, "✓"
	case pipeline.StatusFailed:
		color, icon = red, "✗"
	default:
		color, icon = dim, "·"
	}

	fmt.Print("\r\x1b[K")
	fmt.Printf("%s%s [%s] %s%s\n", color, icon, r.Name, r.Status, reset)
	if r.Err != nil {
		fmt.Printf("  %s└─ %v%s\n", red, r.Err, reset)
	}
}

func (a *App) readLine() (string, error) {
	return a.in.ReadString('\n')
}
