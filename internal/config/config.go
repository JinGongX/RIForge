package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type QuantizeConfig struct {
	Enabled bool `json:"enabled"`

	// Method 目前只约定了 "awq" 这一种，为将来的 gptq / int8 等预留扩展位。
	Method string `json:"method,omitempty"`

	// OutputDir 是相对于 WorkDir 的量化产物目录名，比如 "Qwen2.5-7B-Instruct-AWQ"。
	OutputDir string `json:"output_dir"`

	// Script 是量化脚本路径（相对项目根目录或绝对路径）。
	Script string `json:"script"`

	// PythonBin 允许你指定虚拟环境里的 python 解释器路径，
	// 避免污染系统 python 或者跟 vLLM 的 transformers 版本冲突
	// （参考你踩过的 vllm 0.9.2 与 transformers==5.14.1 不兼容的坑）。
	PythonBin string `json:"python_bin"`

	// ExtraArgs 透传给量化脚本的额外命令行参数，比如 ["--w-bit", "4"]。
	ExtraArgs []string `json:"extra_args,omitempty"`
}

// DeployConfig 描述如何用 docker 把模型跑成一个 vLLM OpenAI 兼容服务。
type DeployConfig struct {
	ContainerName string `json:"container_name"`
	Image         string `json:"image"`

	HostPort      int `json:"host_port"`
	ContainerPort int `json:"container_port"`

	GPUMemoryUtilization float64 `json:"gpu_memory_utilization"`
	MaxModelLen          int     `json:"max_model_len"`

	// DType 例如 "bfloat16"；量化模型通常不需要显式指定。
	DType string `json:"dtype,omitempty"`

	// Quantization 例如 "awq"；只有量化过的权重才应该传这个 flag——
	// 这是你已经踩过的坑："--quantization awq" 配 bf16 权重会直接启动失败。
	Quantization string `json:"quantization,omitempty"`

	// ShmSizeMB 对应 docker run --shm-size，默认给 2048（即 2GB），
	// 避免你遇到过的 Bus error 崩溃。
	ShmSizeMB int `json:"shm_size_mb,omitempty"`

	// ExtraArgs 透传给容器内 vllm 启动命令的额外参数。
	ExtraArgs []string `json:"extra_args,omitempty"`

	// ExtraDockerArgs 透传给 `docker run` 本身的额外参数（在 image 之前插入），
	// 比如 ["--restart", "unless-stopped"]。
	ExtraDockerArgs []string `json:"extra_docker_args,omitempty"`
}

// ModelEntry 是配置文件里 "models" 数组的一项，串起下载 -> 量化 -> 部署三个阶段。
type ModelEntry struct {
	// ID 是这条流水线在 TUI 里显示、以及日志里引用的唯一标识。
	ID string `json:"id"`

	// ModelScopeRepo 是 ModelScope 上的仓库 id，例如 "Qwen/Qwen2.5-7B-Instruct"。
	ModelScopeRepo string `json:"modelscope_repo"`

	// LocalDir 是下载后在 WorkDir 下的目录名。
	LocalDir string `json:"local_dir"`

	Quantize QuantizeConfig `json:"quantize"`
	Deploy   DeployConfig   `json:"deploy"`
}

// Config 是整个配置文件的顶层结构。
type Config struct {
	// WorkDir 是所有模型文件（原始权重 + 量化产物）的根目录，
	// 建议挂载在数据盘上，别放系统盘（模型动辄十几 GB）。
	WorkDir string `json:"work_dir"`

	// ModelScopeBin 是 modelscope CLI 的可执行文件名/路径，默认 "modelscope"。
	ModelScopeBin string `json:"modelscope_bin,omitempty"`

	Models []ModelEntry `json:"models"`
}

// Load 读取并解析 JSON 配置文件，同时做一些基础校验和默认值填充。
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败（检查 JSON 语法）: %w", err)
	}

	if cfg.WorkDir == "" {
		return nil, fmt.Errorf("配置缺少 work_dir")
	}
	if cfg.ModelScopeBin == "" {
		cfg.ModelScopeBin = "modelscope"
	}
	if len(cfg.Models) == 0 {
		return nil, fmt.Errorf("配置里 models 为空，至少需要一条流水线定义")
	}

	seen := make(map[string]bool, len(cfg.Models))
	for i, m := range cfg.Models {
		if m.ID == "" {
			return nil, fmt.Errorf("models[%d] 缺少 id", i)
		}
		if seen[m.ID] {
			return nil, fmt.Errorf("models 里 id 重复: %s", m.ID)
		}
		seen[m.ID] = true

		if m.ModelScopeRepo == "" {
			return nil, fmt.Errorf("model %q 缺少 modelscope_repo", m.ID)
		}
		if m.LocalDir == "" {
			return nil, fmt.Errorf("model %q 缺少 local_dir", m.ID)
		}
		if m.Quantize.Enabled {
			if m.Quantize.OutputDir == "" {
				return nil, fmt.Errorf("model %q 启用了量化但缺少 quantize.output_dir", m.ID)
			}
			if m.Quantize.Script == "" {
				return nil, fmt.Errorf("model %q 启用了量化但缺少 quantize.script", m.ID)
			}
			if m.Quantize.PythonBin == "" {
				cfg.Models[i].Quantize.PythonBin = "python3"
			}
		}
		if m.Deploy.ContainerName == "" {
			return nil, fmt.Errorf("model %q 缺少 deploy.container_name", m.ID)
		}
		if m.Deploy.Image == "" {
			return nil, fmt.Errorf("model %q 缺少 deploy.image", m.ID)
		}
		if m.Deploy.HostPort == 0 {
			return nil, fmt.Errorf("model %q 缺少 deploy.host_port", m.ID)
		}
		if m.Deploy.ContainerPort == 0 {
			cfg.Models[i].Deploy.ContainerPort = 8000
		}
		if m.Deploy.ShmSizeMB == 0 {
			cfg.Models[i].Deploy.ShmSizeMB = 2048
		}

		if m.Deploy.Quantization == "awq" && m.Deploy.DType == "" {
			cfg.Models[i].Deploy.DType = "float16"
		}
	}

	return &cfg, nil
}

// SourcePath 返回某个模型下载后的本地绝对路径。
func (c *Config) SourcePath(m ModelEntry) string {
	return filepath.Join(c.WorkDir, m.LocalDir)
}

// QuantizedPath 返回某个模型量化产物的本地绝对路径（仅在启用量化时有意义）。
func (c *Config) QuantizedPath(m ModelEntry) string {
	return filepath.Join(c.WorkDir, m.Quantize.OutputDir)
}

// DeployPath 返回最终应该挂载进容器里的模型目录：
// 如果启用了量化，部署阶段应该用量化产物；否则用原始下载目录。
func (c *Config) DeployPath(m ModelEntry) string {
	if m.Quantize.Enabled {
		return c.QuantizedPath(m)
	}
	return c.SourcePath(m)
}
