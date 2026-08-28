package pipeline

import (
	"context"
	"fmt"
	"os"

	"igforge/internal/config"
	"igforge/internal/execx"
)

// Downloader 负责"下载模型"这个阶段。
type Downloader struct {
	Cfg *config.Config
}

// AlreadyDownloaded 是幂等检查：只要目标目录存在且里面至少有一个文件，
// 就认为下载已经完成，跳过重新拉取。
//
// 注意：这里特意不用"目录存在"就判定完成——modelscope 下载中断时目录会先被
// 创建出来但内容为空/不全，用"至少有文件"这个更保守的判断能减少误判。
// 如果你要做更严格的校验（比如比对文件清单或 sha256），可以在这个函数里加。
func (d *Downloader) AlreadyDownloaded(m config.ModelEntry) bool {
	dir := d.Cfg.SourcePath(m)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// Run 执行下载。out 用于把 modelscope 的实时输出流式转发给调用方（TUI）。
//
// 命令形态：modelscope download --model <repo> --local_dir <dir>
// 特意用 --local_dir 而不是让 modelscope 用默认缓存路径，是因为你已经踩过的坑：
// 默认缓存会把文件放在 <cache_dir>/<model_id>/snapshots/master/ 这种嵌套路径下，
// 而不是你指定的目录本身。显式传 --local_dir 可以拿到一个"平铺"的目录结构，
// 后面部署阶段直接把这个目录挂载进 vLLM 容器就行，不用再去猜嵌套层级。
func (d *Downloader) Run(ctx context.Context, out chan<- execx.Line, m config.ModelEntry) error {
	dir := d.Cfg.SourcePath(m)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建下载目录失败: %w", err)
	}

	bin := d.Cfg.ModelScopeBin
	if !execx.CommandExists(bin) {
		return fmt.Errorf("找不到可执行文件 %q，请先 pip install modelscope（或在配置里用 modelscope_bin 指定路径）", bin)
	}

	args := []string{
		"download",
		"--model", m.ModelScopeRepo,
		"--local_dir", dir,
	}

	out <- execx.Line{Text: fmt.Sprintf("$ %s %s", bin, joinArgs(args))}
	return execx.Run(ctx, out, "", bin, args...)
}

func joinArgs(args []string) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += a
	}
	return s
}
