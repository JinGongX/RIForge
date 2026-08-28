package execx

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
)

// Line 是从子进程输出流里读到的一行日志，Stderr 标记它来自标准错误还是标准输出
// （TUI 侧可以用这个来给 stderr 行上不同颜色，方便一眼看出报错）。
type Line struct {
	Text     string
	Stderr   bool
	Progress bool
}

// Run 启动 cmd，把它的 stdout/stderr 按行发送到 out channel（out 由调用方创建和关闭外的
// 消费循环负责读取，Run 不会关闭 out，方便一次运行里复用同一个 channel 串联多条命令）。
//
// ctx 用来支持取消（比如用户在 TUI 里按 Ctrl+C 中途终止某个阶段）。
// 返回值是命令的最终错误（非 0 退出码会被包装成 error）。
func Run(ctx context.Context, out chan<- Line, dir string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout 管道失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建 stderr 管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动命令失败 (%s): %w", name, err)
	}

	// 两个 goroutine 分别拉 stdout / stderr，用 done channel 等它们都读完，
	// 避免其中一个管道缓冲区打满导致子进程卡死（这是很多人第一次写这种封装时
	// 会踩的坑：只处理 stdout，stderr 缓冲区满了子进程就永远 block 在写入上）。
	doneOut := make(chan struct{})
	doneErr := make(chan struct{})
	go pump(stdout, out, false, doneOut)
	go pump(stderr, out, true, doneErr)
	<-doneOut
	<-doneErr

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("命令执行失败 (%s): %w", name, err)
	}
	return nil
}

func pump(r io.Reader, out chan<- Line, isErr bool, done chan<- struct{}) {
	defer close(done)

	reader := bufio.NewReaderSize(r, 64*1024)
	buf := make([]byte, 0, 256)

	flush := func(delim byte) {
		if len(buf) == 0 {
			// \r\n 这种组合会先在 \r 处切出内容，紧接着 \n 处切出空字符串——
			// 这个空字符串没有信息量，直接丢弃，避免打印出一堆空行。
			return
		}
		out <- Line{Text: string(buf), Stderr: isErr, Progress: delim == '\r'}
		buf = buf[:0]
	}

	for {
		b, err := reader.ReadByte()
		if err != nil {
			flush('\n') // 流结束时如果还有残留内容（没有以 \r/\n 结尾），按普通行处理
			return
		}
		switch b {
		case '\n', '\r':
			flush(b)
		default:
			buf = append(buf, b)
			// 防止极端情况下单行无限增长把内存吃爆（比如某个工具异常输出
			// 了一整段没有任何换行符的超长文本），超过 1MB 强制切断。
			if len(buf) >= 1024*1024 {
				flush('\n')
			}
		}
	}
}

// CommandExists 检查某个可执行文件是否在 PATH 里，用于运行前的前置检查
// （比如提前告诉用户"没装 docker"，而不是等命令跑起来才报错）。
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
