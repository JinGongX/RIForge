# RIForge

```
    ____  _ ____
   / __ \(_) __/___  _________ ____
  / /_/ / / /_/ __ \/ ___/ __ `/ _ \
 / _, _/ / __/ /_/ / /  / /_/ /  __/
/_/ |_/_/_/  \____/_/   \__, /\___/
                       /____/
```

> **LLM Runtime & Inference Forge** —— 基于 Golang + TUI 的大模型推理服务自动化部署工具

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Status](https://img.shields.io/badge/Status-Active%20Development-yellow)]()
[![Visitors](https://api.visitorbadge.io/api/visitors?path=https%3A%2F%2Fgithub.com%2FJinGongX%2FRIForge&countColor=%23f47373&style=flat)](https://visitorbadge.io/status?path=https%3A%2F%2Fgithub.com%2FJinGongX%2FRIForge)

把 **模型下载 → 模型量化 → Docker/vLLM 部署** 串成一条可重复执行的自动化 Pipeline，让一个「模型文件」在几分钟内变成一个可以直接调用的 **OpenAI Compatible API**，从此告别手写一长串 `modelscope` / `docker run` 命令和反复踩坑。

---

## ✨ 核心特性

- **终端交互 TUI** —— 纯 Go 标准库实现，零第三方依赖；模型列表、阶段状态、实时彩色日志一屏掌控，Ctrl+C 优雅中断。
- **JSON 配置驱动** —— 一个 `models.json` 描述全部流水线，启动即校验，缺字段 / 重复 ID 直接报错。
- **ModelScope 下载** —— 一键拉取模型到指定目录，规避默认缓存嵌套路径的坑。
- **AutoAWQ 量化（可选）** —— 按需启用、脚本化执行，自动处理 vLLM 兼容的 tokenizer 格式。
- **Docker + vLLM 一键部署** —— GPU / 显存 / 端口 / 挂载等参数固化封装，启动即 OpenAI Compatible API。
- **幂等可重跑** —— 已下载、已量化、运行中的容器自动跳过；改了参数可强制重建。

## 🔁 Pipeline 工作流

每条模型配置都是一条独立流水线，由 Runner 统一编排，三个阶段可随时单独重跑：

```
┌───────────────┐   ┌───────────────┐   ┌──────────────────┐
│  ① Download   │ → │ ② Quantize    │ → │ ③ Deploy         │
│   ModelScope  │   │  AutoAWQ (可选) │   │  Docker + vLLM   │
└───────────────┘   └───────────────┘   └──────────────────┘
        │                   │                    │
   原始权重文件          AWQ 量化产物        OpenAI Compatible API
                                        http://<host>:<port>/v1
```

部署阶段内置的工程化细节（默认行为，无需手写）：

- `--gpus all` 传入 GPU；`--shm-size` 默认 2GB，避免 Bus error
- 模型目录**只读挂载**进容器，容器无权改动模型文件
- `--quantization awq` 仅在明确配置时传入，避免 bf16 权重误传导致启动失败
- 自动带上 `--served-model-name`、`--port`、`--gpu-memory-utilization`、`--max-model-len` 等 vLLM 参数

## 🚀 快速开始

### 环境要求

| 依赖 | 用途 | 说明 |
| --- | --- | --- |
| Go ≥ 1.22 | 构建 | 仅标准库，无第三方依赖 |
| modelscope CLI | 下载模型 | `pip install modelscope` |
| Python 3 + autoawq | 量化（可选） | 仅启用量化时所需 `pip install autoawq` or `pip3 install autoawq -i https://pypi.tuna.tsinghua.edu.cn/simple --no-cache-dir` |
| Docker + NVIDIA Container Toolkit | 部署 | 需要 NVIDIA GPU && `docker pull vllm/vllm-openai:v0.9.2`  |
| vLLM 镜像 | 推理运行时 | 如 `vllm/vllm-openai:v0.9.2` |

```
完整步骤：
1、pip install modelscope
2、pip install autoawq or pip3 install autoawq -i https://pypi.tuna.tsinghua.edu.cn/simple --no-cache-dir
3、docker pull vllm/vllm-openai:v0.9.2
环境配置后直接运行程序
启动rigorge
go build -o riforge ./cmd/riforge
./riforge -config configs/models.json
再选择流水线即可
```
### 构建 & 运行

```bash
# 1. 准备配置（按需修改 work_dir 与模型列表）
cp configs/models.json configs/my-models.json

# 2. 构建
go build -o riforge ./cmd/riforge

# 3. 启动 TUI：输入编号执行对应流水线，输入 q 退出
./riforge -config configs/my-models.json
```

运行效果（示意）：

```
可用流水线：
   1) qwen2.5-7b-instruct-bf16         [无量化] 端口 8000
       下载:○ 未完成  量化:✓ 已就绪  部署:○ 未部署
   2) qwen2.5-7b-instruct-awq          [量化:awq] 端口 8002
       下载:○ 未完成  量化:○ 未完成  部署:○ 未部署
   3) llama3-8b-instruct-bf16          [无量化] 端口 8004
       下载:○ 未完成  量化:✓ 已就绪  部署:○ 未部署
   ...
输入模型编号执行流水线，输入 q 退出： 2
```

### 调用推理服务

流水线跑完后，服务即已就绪，直接按 OpenAI 格式调用：

```bash
curl http://localhost:8002/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2.5-7b-instruct-awq",
    "messages": [{"role": "user", "content": "你好，介绍一下你自己"}]
  }'
```

## ⚙️ 配置说明

以 `configs/models.json` 为例（下方为带注释的示意，实际文件为合法 JSON）：

```jsonc
{
  "work_dir": "/data/models",            // 所有模型文件的根目录（建议放数据盘）
  "modelscope_bin": "modelscope",        // modelscope CLI 路径（可选，默认 "modelscope"）
  "models": [
    {
      "id": "qwen2.5-7b-instruct-awq",               // 流水线唯一标识，同时作为 served-model-name
      "modelscope_repo": "Qwen/Qwen2.5-7B-Instruct", // ModelScope 仓库
      "local_dir": "Qwen2.5-7B-Instruct",            // 下载后的存放目录
      "quantize": {                                  // 量化阶段（不启用可整块省略）
        "enabled": true,
        "method": "awq",
        "output_dir": "Qwen2.5-7B-Instruct-AWQ",
        "script": "scripts/quantize_awq.py",
        "python_bin": "python3"
      },
      "deploy": {                                    // Docker + vLLM 部署
        "container_name": "vllm-qwen25-7b-awq",
        "image": "vllm/vllm-openai:v0.9.2",
        "host_port": 8002,
        "gpu_memory_utilization": 0.4,
        "max_model_len": 4096,
        "quantization": "awq",
        "dtype": "float16"
      }
    }
  ]
}
```

**deploy 字段速览**：`container_name` 容器名 · `image` vLLM 镜像 · `host_port` 宿主机端口 · `gpu_memory_utilization` 显存利用率 · `max_model_len` 最大上下文 · `dtype` 精度（量化模型默认 `float16`）· `quantization` 量化格式（如 `awq`）· `extra_args` 追加 vLLM 参数 · `extra_docker_args` 追加 `docker run` 参数。

## 📁 项目结构

```
RIForge/
├── cmd/riforge/             # 程序入口
├── internal/
│   ├── config/              # 配置加载与校验
│   ├── execx/               # 子进程执行与日志流式转发
│   ├── pipeline/            # Runner 编排 + 下载/量化/部署三阶段
│   └── tui/                 # 终端交互界面
├── scripts/quantize_awq.py  # AutoAWQ 量化脚本
├── configs/models.json      # 示例配置（内置 4 条流水线）
├── go.mod
└── README.md
```

**设计要点**：Pipeline 通过 `StageUpdate` 事件流实时上报进度，TUI 与未来要加的 HTTP API / Web 控制台可以复用同一条通道；每个阶段相互独立、可替换（例如把 Docker 部署换成 Kubernetes Provider 时，下载与量化阶段完全不用改）。

## 🗺️ Roadmap

**已完成**

- [x] Golang TUI 交互 · JSON 配置 + 校验
- [x] ModelScope 模型下载 · AutoAWQ 量化 · Docker + vLLM 部署
- [x] 幂等检查 · 强制重建 · Ctrl+C 优雅中断 · 实时彩色日志

**规划中**

- [ ] Kubernetes 自动部署 · GPU 资源调度 · NVIDIA GPU Operator 集成
- [ ] AI Gateway 服务注册与统一路由
- [ ] 可观测性：Prometheus / DCGM Exporter / Grafana
- [ ] Vue3 管理控制台（TUI / Web 双管理入口）

最终愿景：让模型部署像安装软件一样简单，让 GPU 推理服务像 Kubernetes Workload 一样可管理 —— 一套轻量、可扩展的 **LLM Inference Infrastructure Platform**。

## 📄 License

[MIT](LICENSE)
