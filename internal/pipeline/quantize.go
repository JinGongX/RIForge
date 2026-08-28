package pipeline

import (
	"context"
	"fmt"
	"os"

	"igforge/internal/config"
	"igforge/internal/execx"
)

// Quantizer 负责"量化模型"这个阶段。
type Quantizer struct {
	Cfg *config.Config
}

// AlreadyQuantized 幂等检查：量化产物目录存在且非空就跳过。
func (q *Quantizer) AlreadyQuantized(m config.ModelEntry) bool {
	if !m.Quantize.Enabled {
		return true // 没启用量化的模型，这一步天然"已完成"（无需执行）
	}
	dir := q.Cfg.QuantizedPath(m)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// Run 执行量化脚本。约定的调用形态（跟 scripts/quantize_awq.py 对齐）：
//
//	<python_bin> <script> --model-path <下载目录> --output-path <量化产物目录> [extra_args...]
//
// 如果你有自己已经写好的量化脚本、参数形式不一样，改这里的 args 拼接就行，
// 其余幂等检查、日志流式转发的逻辑不用动。
func (q *Quantizer) Run(ctx context.Context, out chan<- execx.Line, m config.ModelEntry) error {
	if !m.Quantize.Enabled {
		out <- execx.Line{Text: "该模型未启用量化，跳过"}
		return nil
	}

	srcDir := q.Cfg.SourcePath(m)
	if _, err := os.Stat(srcDir); err != nil {
		return fmt.Errorf("量化的输入目录不存在，请确认下载阶段已完成: %s", srcDir)
	}

	outDir := q.Cfg.QuantizedPath(m)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("创建量化输出目录失败: %w", err)
	}

	if !execx.CommandExists(m.Quantize.PythonBin) {
		return fmt.Errorf("找不到 python 解释器 %q，检查配置里的 quantize.python_bin", m.Quantize.PythonBin)
	}
	if _, err := os.Stat(m.Quantize.Script); err != nil {
		return fmt.Errorf("量化脚本不存在: %s", m.Quantize.Script)
	}

	args := []string{
		m.Quantize.Script,
		"--model-path", srcDir,
		"--output-path", outDir,
	}
	args = append(args, m.Quantize.ExtraArgs...)

	out <- execx.Line{Text: fmt.Sprintf("$ %s %s", m.Quantize.PythonBin, joinArgs(args))}
	return execx.Run(ctx, out, "", m.Quantize.PythonBin, args...)
}
