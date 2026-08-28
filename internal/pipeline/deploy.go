package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"igforge/internal/config"
	"igforge/internal/execx"
)

type Deployer struct {
	Cfg *config.Config
}

// ContainerState 描述容器当前的状态，用于幂等判断。
type ContainerState int

const (
	ContainerAbsent ContainerState = iota
	ContainerRunning
	ContainerStoppedButExists
)

func (d *Deployer) Inspect(ctx context.Context, m config.ModelEntry) (ContainerState, error) {
	if !execx.CommandExists("docker") {
		return ContainerAbsent, fmt.Errorf("找不到 docker 命令，请确认已安装且当前用户有权限调用")
	}

	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Running}}", m.Deploy.ContainerName)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		// docker inspect 在容器不存在时会以非 0 退出码返回，这是预期情况，不算错误。
		// 注意：不同 Docker 版本/平台（Docker Desktop for Mac、Linux 上的 dockerd
		// 等）返回的错误文案大小写不完全一致，例如 "No such object" vs
		// "no such object"，这里用小写比较做大小写不敏感匹配，避免误判成真错误。
		if strings.Contains(strings.ToLower(stderr.String()), "no such object") {
			return ContainerAbsent, nil
		}
		return ContainerAbsent, fmt.Errorf("查询容器状态失败: %w (%s)", err, stderr.String())
	}

	if strings.TrimSpace(stdout.String()) == "true" {
		return ContainerRunning, nil
	}
	return ContainerStoppedButExists, nil
}

// Run 启动（或在容器已存在但停止时，重启）vLLM 容器。
//
// 已经在运行中的同名容器会被跳过（幂等）；调用方如果想强制重建，
// 应该先显式调用 Remove。
func (d *Deployer) Run(ctx context.Context, out chan<- execx.Line, m config.ModelEntry) error {
	modelDir := d.Cfg.DeployPath(m)
	if _, err := os.Stat(modelDir); err != nil {
		return fmt.Errorf("要挂载的模型目录不存在，请确认前置阶段已完成: %s", modelDir)
	}

	args := d.buildRunArgs(m, modelDir)
	out <- execx.Line{Text: fmt.Sprintf("$ docker %s", joinArgs(args))}
	return execx.Run(ctx, out, "", "docker", args...)
}

// Remove 停止并删除同名容器，用于"重新部署"场景（比如改了 gpu_memory_utilization
// 想重跑）。
func (d *Deployer) Remove(ctx context.Context, out chan<- execx.Line, m config.ModelEntry) error {
	out <- execx.Line{Text: fmt.Sprintf("$ docker rm -f %s", m.Deploy.ContainerName)}
	return execx.Run(ctx, out, "", "docker", "rm", "-f", m.Deploy.ContainerName)
}

// buildRunArgs 拼装完整的 `docker run` 参数列表。把这些已经验证过的经验点
// 都固化成默认行为，避免每次手写命令都要重新想一遍：
//   - --gpus all：把 GPU 传进容器（k3s 场景对应 device plugin，这里是最简单的
//     单机 docker 场景，直接用 nvidia-container-toolkit 的 --gpus all）
//   - --shm-size：默认 2GB，对应你踩过的 Bus error 崩溃
//   - -v modelDir:/model:ro：只读挂载，容器不应该有权限改动模型文件
//   - --quantization 只在配置里显式给了 Quantization 字段时才加，
//     避免 bf16 权重被误传 --quantization awq 导致启动失败
func (d *Deployer) buildRunArgs(m config.ModelEntry, modelDir string) []string {
	dep := m.Deploy

	args := []string{
		"run", "-d",
		"--name", dep.ContainerName,
		"--gpus", "all",
		"--shm-size", fmt.Sprintf("%dm", dep.ShmSizeMB),
		"-p", fmt.Sprintf("%d:%d", dep.HostPort, dep.ContainerPort),
		"-v", modelDir + ":/model:ro",
	}
	args = append(args, dep.ExtraDockerArgs...)
	args = append(args, dep.Image)

	// 以下是传给容器里 vllm 启动命令（vllm-openai 镜像的 entrypoint 已经是
	// `vllm serve`，这里只需要拼它的参数）的部分。
	vllmArgs := []string{
		"--model", "/model",
		"--served-model-name", m.ID,
		"--port", strconv.Itoa(dep.ContainerPort),
	}
	if dep.MaxModelLen > 0 {
		vllmArgs = append(vllmArgs, "--max-model-len", strconv.Itoa(dep.MaxModelLen))
	}
	if dep.GPUMemoryUtilization > 0 {
		vllmArgs = append(vllmArgs, "--gpu-memory-utilization", strconv.FormatFloat(dep.GPUMemoryUtilization, 'f', -1, 64))
	}
	if dep.DType != "" {
		vllmArgs = append(vllmArgs, "--dtype", dep.DType)
	}
	if dep.Quantization != "" {
		vllmArgs = append(vllmArgs, "--quantization", dep.Quantization)
	}
	vllmArgs = append(vllmArgs, dep.ExtraArgs...)

	return append(args, vllmArgs...)
}
