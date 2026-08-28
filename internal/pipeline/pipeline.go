package pipeline

import (
	"context"

	"igforge/internal/config"
	"igforge/internal/execx"
)

// Runner 编排单个 ModelEntry 的完整生命周期：下载 -> 量化 -> 部署。
// 三个阶段中任何一个失败都会立刻中止后续阶段（部署一个没量化完整的模型
// 没有意义），但已经跑完/跳过的阶段结果会原样保留在返回的 []StageResult 里，
// 方便 TUI 显示"到底是哪一步炸的"。
type Runner struct {
	Cfg        *config.Config
	Downloader *Downloader
	Quantizer  *Quantizer
	Deployer   *Deployer
}

// NewRunner 用给定配置构造一个 Runner，内部三个阶段共享同一份 Cfg。
func NewRunner(cfg *config.Config) *Runner {
	return &Runner{
		Cfg:        cfg,
		Downloader: &Downloader{Cfg: cfg},
		Quantizer:  &Quantizer{Cfg: cfg},
		Deployer:   &Deployer{Cfg: cfg},
	}
}

// StageUpdate 是 Runner 在执行过程中往外推送的事件：既可能是某个阶段的
// 状态变化（Result 非空），也可能是某个阶段内部子进程输出的一行日志
// （Log 非空）。TUI 只需要 select 这一个 channel 就能拿到全部实时信息。
type StageUpdate struct {
	Result *StageResult
	Log    *execx.Line
}

// RunAll 依次执行三个阶段，通过 updates channel 实时上报进度，函数返回时
// updates 会被 close，调用方可以用 range 消费直到自然结束。
//
// forceRedeploy 为 true 时会先删除同名容器再重新创建（用于"改了部署参数
// 想重跑"的场景），为 false 时如果容器已在运行则直接跳过部署阶段。
func (r *Runner) RunAll(ctx context.Context, m config.ModelEntry, forceRedeploy bool, updates chan<- StageUpdate) {
	defer close(updates)

	logCh := make(chan execx.Line)
	// 把 logCh 的内容原样转发进 updates，跟阶段状态事件共用一条时间线，
	// 这样 TUI 侧看到的日志顺序就是真实的执行顺序。
	logDone := make(chan struct{})
	go func() {
		defer close(logDone)
		for line := range logCh {
			l := line
			updates <- StageUpdate{Log: &l}
		}
	}()

	report := func(name StageName, status Status, err error) {
		updates <- StageUpdate{Result: &StageResult{Name: name, Status: status, Err: err}}
	}

	// ---------- 阶段一：下载 ----------
	if r.Downloader.AlreadyDownloaded(m) {
		report(StageDownload, StatusSkipped, nil)
	} else {
		report(StageDownload, StatusRunning, nil)
		if err := r.Downloader.Run(ctx, logCh, m); err != nil {
			report(StageDownload, StatusFailed, err)
			close(logCh)
			<-logDone
			return
		}
		report(StageDownload, StatusSuccess, nil)
	}

	// ---------- 阶段二：量化 ----------
	if r.Quantizer.AlreadyQuantized(m) {
		report(StageQuantize, StatusSkipped, nil)
	} else {
		report(StageQuantize, StatusRunning, nil)
		if err := r.Quantizer.Run(ctx, logCh, m); err != nil {
			report(StageQuantize, StatusFailed, err)
			close(logCh)
			<-logDone
			return
		}
		report(StageQuantize, StatusSuccess, nil)
	}

	// ---------- 阶段三：部署 ----------
	state, err := r.Deployer.Inspect(ctx, m)
	if err != nil {
		report(StageDeploy, StatusFailed, err)
		close(logCh)
		<-logDone
		return
	}

	switch {
	case state == ContainerRunning && !forceRedeploy:
		report(StageDeploy, StatusSkipped, nil)
	default:
		if state != ContainerAbsent {
			// 容器存在（不管是运行中还是已停止），先清掉再重建，
			// 避免 docker run 因为容器名冲突直接报错退出。
			if err := r.Deployer.Remove(ctx, logCh, m); err != nil {
				report(StageDeploy, StatusFailed, err)
				close(logCh)
				<-logDone
				return
			}
		}
		report(StageDeploy, StatusRunning, nil)
		if err := r.Deployer.Run(ctx, logCh, m); err != nil {
			report(StageDeploy, StatusFailed, err)
			close(logCh)
			<-logDone
			return
		}
		report(StageDeploy, StatusSuccess, nil)
	}

	close(logCh)
	<-logDone
}
