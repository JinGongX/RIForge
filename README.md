# Riforge

```
    ____  _ ____                    
   / __ \(_) __/___  _________ ____ 
  / /_/ / / /_/ __ \/ ___/ __ `/ _ \
 / _, _/ / __/ /_/ / /  / /_/ /  __/
/_/ |_/_/_/  \____/_/   \__, /\___/ 
                       /____/       
```

> 模型下载 / 量化 / 部署 自动化工具

RIForge — LLM Runtime & Inference Forge

一个基于 Golang + TUI 构建的大模型推理服务自动化部署工具。

RIForge 致力于将：

模型下载 → 模型量化 → Docker 镜像/容器部署 → vLLM 推理服务

串联成一条自动化 Pipeline，让大模型从“模型文件”快速变成一个可以直接调用的 OpenAI Compatible API。

未来将进一步扩展：

Kubernetes / GPU 调度 / AI Gateway / 服务注册 / 可观测性 / Vue3 管理控制台

最终形成一套轻量级、可扩展的 LLM Inference Infrastructure Platform。

⸻

✨ Features

当前已实现

* Golang TUI 交互
* JSON 配置
* 配置校验
* 模型下载
* AWQ 模型量化
* Docker 自动部署


规划中

* Kubernetes 自动部署
* GPU 资源调度
* NVIDIA GPU Operator 集成
* AI Gateway 服务注册
* 模型服务自动注册
* Prometheus / DCGM Exporter
* Grafana Dashboard
* 服务健康检查
* 模型运行状态监控
* Vue3 Web 管理控制台
* TUI / Web 双管理入口
* 一键安装完整 AI Inference Stack

⸻

🎯 为什么需要 RIForge？

部署一个大模型推理服务，看起来只是： 1

下载模型
    ↓
量化模型
    ↓
启动 vLLM

但实际部署过程中通常还需要处理：

模型下载
    ↓
模型格式检查
    ↓
量化
    ↓
模型目录管理
    ↓
CUDA / Driver / vLLM 兼容
    ↓
Docker GPU Runtime
    ↓
显存配置
    ↓
vLLM 参数配置
    ↓
端口映射
    ↓
服务启动
    ↓
健康检查
    ↓
Gateway 注册
    ↓
监控

这些操作如果全部手动执行，不仅步骤多，而且非常容易出现：

* 模型路径错误
* Docker GPU 参数错误
* vLLM 参数不一致
* 量化环境污染
* 模型重复下载
* 容器端口冲突
* GPU 显存配置不合理
* 服务启动失败后难以定位

RIForge 希望解决的就是这一层问题：

把“大模型推理服务部署”从一系列命令，变成一个可重复执行的 Pipeline。

⸻

🏗 Architecture

当前版本采用简单、低依赖的分层设计。

                    ┌─────────────────────┐
                    │      RIForge TUI    │
                    │                     │
                    │  Model Selection     │
                    │  Pipeline Control    │
                    └──────────┬──────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │      Pipeline       │
                    │                     │
                    │  Download           │
                    │      ↓              │
                    │  Quantize           │
                    │      ↓              │
                    │  Deploy             │
                    └──────────┬──────────┘
                               │
             ┌─────────────────┼─────────────────┐
             ▼                 ▼                 ▼
       Model Download       AutoAWQ          Docker/vLLM
             │                 │                 │
             ▼                 ▼                 ▼
        Model Files        AWQ Model       Inference API

RIForge 当前核心 Pipeline：

Download
   ↓
Quantize
   ↓
Deploy

每个阶段都可以独立执行，同时由 Pipeline 负责统一编排。

⸻

📁 Project Structure

RIForge/
├── cmd/
│   └── riforge/
│       └── main.go
│
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── validate.go
│   │
│   ├── execx/
│   │   └── exec.go
│   │
│   ├── pipeline/
│   │   ├── pipeline.go
│   │   ├── download.go
│   │   ├── quantize.go
│   │   └── deploy.go
│   │
│   └── tui/
│       └── tui.go
│
├── scripts/
│   └── quantize_awq.py
│
├── configs/
│   └── models.json
│
├── go.mod
├── go.sum
└── README.md

⸻

🔍 Core Modules

cmd/riforge

程序入口。

cmd/riforge/main.go

主要负责：

1. 加载配置
2. 配置校验
3. 创建 Context
4. 监听 Ctrl+C
5. 启动 TUI
6. 启动 Pipeline

整体生命周期：

main
 │
 ├── Load Config
 │
 ├── Validate Config
 │
 ├── Create Context
 │
 ├── Handle SIGINT
 │
 └── Start TUI
       │
       └── Execute Pipeline

⸻

⚙️ internal/config

负责 RIForge 的配置模型和配置校验。

目前使用 JSON 而不是 YAML。

原因很简单：

RIForge 当前定位是一个轻量级 CLI/TUI 工具，希望尽可能减少外部依赖。

例如：

{
  "models": [
    {
      "name": "qwen2.5-7b-bf16",
      "model_path": "/data/models/Qwen2.5-7B-Instruct",
      "port": 8000,
      "dtype": "bfloat16"
    },
    {
      "name": "qwen2.5-7b-awq",
      "model_path": "/data/models/Qwen2.5-7B-Instruct-AWQ",
      "port": 8002,
      "quantization": "awq"
    }
  ]
}

配置层负责：

JSON
 ↓
Unmarshal
 ↓
Validate
 ↓
Runtime Config

避免 Pipeline 内部直接依赖 JSON 结构。

⸻

🚀 internal/execx

RIForge 中比较核心的基础设施模块。

负责统一封装：

启动子进程 + 实时输出 stdout/stderr + Context 取消

因为模型下载、量化、Docker 部署本质上都会执行外部程序。

例如：

Download
   └── python / huggingface-cli
Quantize
   └── python quantize_awq.py
Deploy
   └── docker run

因此没有必要在三个 Pipeline 中分别实现：

exec.Command(...)
cmd.Stdout = ...
cmd.Stderr = ...
cmd.Run()

而是统一抽象成：

execx
  │
  ├── Command
  ├── Context
  ├── stdout streaming
  ├── stderr streaming
  └── cancellation

这样三个阶段可以共享同一套执行机制。

⸻

🔄 internal/pipeline

RIForge 的核心。

当前 Pipeline 分为三个阶段：

┌─────────────┐
│   Download  │
└──────┬──────┘
       ↓
┌─────────────┐
│   Quantize  │
└──────┬──────┘
       ↓
┌─────────────┐
│    Deploy   │
└─────────────┘

⸻

1. Download

pipeline/download.go

负责模型下载。

目标是将远程模型转换成：

Local Model Directory

例如：

/data/models/
└── Qwen2.5-7B-Instruct/
    ├── config.json
    ├── tokenizer.json
    ├── tokenizer_config.json
    ├── model-00001-of-00004.safetensors
    ├── ...
    └── ...

⸻

2. Quantize

pipeline/quantize.go

负责模型量化。

目前主要支持：

FP16 / BF16
      ↓
     AWQ

量化脚本：

scripts/quantize_awq.py

Go Pipeline 负责调度 Python Script。

Python Script 负责：

Load Model
    ↓
Calibration
    ↓
AWQ Quantization
    ↓
Save Quantized Model

最终生成：

/data/models/
└── Qwen2.5-7B-Instruct-AWQ/

这种设计将：

Go = Orchestration

和：

Python = Model Processing

进行了分离。

后续可以非常容易扩展：

AWQ
GPTQ
GGUF
FP8
INT8
...

⸻

3. Deploy

pipeline/deploy.go

负责通过 Docker 部署 vLLM。

最终生成一个可以直接访问的：

OpenAI Compatible API

例如：

curl http://localhost:8002/v1/chat/completions

请求：

{
  "model": "qwen2.5-7b-instruct-awq",
  "messages": [
    {
      "role": "user",
      "content": "你好"
    }
  ]
}

因此从用户角度来看：

RIForge
   ↓
选择模型
   ↓
自动下载
   ↓
自动量化
   ↓
自动 Docker 部署
   ↓
得到 OpenAI Compatible API

⸻

🖥 TUI

RIForge 当前 TUI 完全基于：

Go Standard Library + ANSI Escape Sequence

没有引入：

Bubble Tea
tview
Cobra

等第三方 UI 框架。

核心目标是：

保持 RIForge 的轻量性和可控性。

当前 TUI 可以承担：

┌───────────────────────────────────────────┐
│                 RIForge                   │
│      LLM Runtime & Inference Forge        │
├───────────────────────────────────────────┤
│                                           │
│  Select Model                             │
│                                           │
│  > Qwen2.5-7B-Instruct                    │
│    Qwen2.5-7B-Instruct-AWQ                │
│                                           │
│  Pipeline                                 │
│                                           │
│  [✓] Download                             │
│  [✓] Quantize                             │
│  [✓] Deploy                               │
│                                           │
├───────────────────────────────────────────┤
│  Logs                                     │
│                                           │
│  Downloading model...                     │
│  Quantizing model...                      │
│  Starting vLLM...                         │
│                                           │
└───────────────────────────────────────────┘

后续如果 TUI 复杂度明显提升，再考虑引入专门的 TUI Framework。

⸻

🐍 AutoAWQ

RIForge 当前提供：

scripts/quantize_awq.py

用于 AWQ 量化。

Go 不直接实现量化算法，而是负责：

Go
 │
 │ execute
 ▼
Python
 │
 ▼
AutoAWQ
 │
 ▼
AWQ Model

这样既能够利用 Python AI 生态，又不会让整个项目变成一个 Python 项目。

⸻

📦 Model Configuration

示例配置：

configs/models.json

目前用于配置两个实例：

Qwen2.5-7B BF16
       │
       └── :8000
Qwen2.5-7B AWQ
       │
       └── :8002

最终可以同时运行：

                 ┌──────────────────┐
                 │      RIForge     │
                 └────────┬─────────┘
                          │
             ┌────────────┴────────────┐
             ▼                         ▼
      BF16 Inference             AWQ Inference
          :8000                     :8002
             │                         │
             ▼                         ▼
          vLLM                      vLLM

这个设计也为后续的：

Gateway
   ↓
Model Registry
   ↓
Multiple vLLM Instances

提供了基础。

⸻

🚀 Quick Start

Requirements

建议运行环境：

Linux
Docker
NVIDIA Driver
NVIDIA Container Toolkit
Python 3.x
Go 1.27+
NVIDIA GPU

同时需要保证：

NVIDIA Driver
      ↓
CUDA
      ↓
Docker NVIDIA Runtime
      ↓
vLLM

版本之间能够正常兼容。

⸻

Build

git clone <your-repository>
cd RIForge
go build -o riforge ./cmd/riforge

运行：

./riforge

⸻

🔧 Pipeline Example

例如选择：

Qwen2.5-7B-Instruct

RIForge 执行：

Step 1 — Download

Downloading model...
████████████████████ 100%

↓

Step 2 — Quantize

Quantizing model...
Loading model...
Calibrating...
Quantizing...
Saving AWQ model...
Done.

↓

Step 3 — Deploy

Starting Docker container...
vLLM starting...
INFO: API server started
INFO: Uvicorn running on 0.0.0.0:8002

↓

最终：

http://localhost:8002

即可提供 OpenAI Compatible API。

⸻

🧩 Design Philosophy

RIForge 当前遵循几个核心原则。

1. Orchestration First

RIForge 不负责重新实现：

HuggingFace
AutoAWQ
vLLM
Docker
Kubernetes
Prometheus

而是负责把这些能力：

组合
编排
配置
自动化

形成完整的部署流程。

⸻

2. CLI First

首先保证：

命令行
TUI

能够独立完成部署。

这样即使没有 Web UI，也可以直接在服务器上使用。

⸻

3. Infrastructure as Pipeline

将模型部署抽象为：

Pipeline

而不是：

一堆 Shell Command

因此未来可以自然扩展：

Download
    ↓
Quantize
    ↓
Build
    ↓
Deploy
    ↓
Register
    ↓
Observe

⸻

🗺 Roadmap

RIForge 后续计划逐步从：

TUI Deployment Tool

演进成：

LLM Inference Platform

⸻

Phase 1 — Local Runtime

当前阶段

                 RIForge
                    │
        ┌───────────┼───────────┐
        ▼           ▼           ▼
     Download    Quantize     Deploy
                                │
                                ▼
                               vLLM

目标：

一键完成模型 → 推理服务。

⸻

Phase 2 — Kubernetes

下一阶段引入：

Kubernetes
NVIDIA Device Plugin
GPU Operator

Pipeline 从：

Docker Run

扩展为：

Docker
  │
  └── Kubernetes

最终：

RIForge
   │
   ▼
Kubernetes
   │
   ├── Deployment
   ├── Service
   ├── ConfigMap
   └── GPU Resource
          │
          ▼
       NVIDIA GPU

⸻

Phase 3 — AI Gateway

当模型实例越来越多：

vLLM-1
vLLM-2
vLLM-3
vLLM-4

需要统一入口：

                 AI Gateway
                     │
          ┌──────────┼──────────┐
          ▼          ▼          ▼
       vLLM-1     vLLM-2     vLLM-3

RIForge 将负责：

部署服务
   ↓
服务注册
   ↓
Gateway Registration
   ↓
统一 API

例如：

/v1/chat/completions

由 Gateway 根据：

model
tenant
load
GPU
health

进行路由。

⸻

Phase 4 — Observability

增加：

Prometheus
DCGM Exporter
Grafana

建立完整的 GPU / Inference Observability。

重点指标包括：

GPU Utilization
GPU Memory
Temperature
Power
Requests
Tokens
TTFT
TPOT
Throughput
Latency
Error Rate

最终：

                 RIForge
                    │
                    ▼
              Inference Stack
                    │
          ┌─────────┴─────────┐
          ▼                   ▼
       vLLM                 Gateway
          │                   │
          └─────────┬─────────┘
                    ▼
                Metrics
                    │
             ┌──────┴──────┐
             ▼             ▼
         Prometheus      Grafana

⸻

Phase 5 — Vue3 Management Console

最终增加：

Vue3
TypeScript
Vite

管理控制台。

TUI 负责：

Server / DevOps

Web Console 负责：

Platform Management

例如：

┌──────────────────────────────────────────────┐
│ RIForge Console                              │
├────────────┬─────────────────────────────────┤
│ Dashboard  │ GPU                             │
│ Models     │                                 │
│ Instances  │ GPU 0     ███████░░  78%        │
│ Deploy     │ GPU 1     ████░░░░░  42%        │
│ Gateway    │                                 │
│ Monitor    │ Inference                       │
│ Settings   │ Requests   12,321               │
│            │ Tokens     2.31M                │
│            │ P95        183ms                │
└────────────┴─────────────────────────────────┘

最终形成：

                Vue3 Console
                     │
                     ▼
                 RIForge API
                     │
          ┌──────────┼──────────┐
          ▼          ▼          ▼
       Models     Gateway    Monitor
          │          │          │
          └──────────┼──────────┘
                     ▼
                Kubernetes
                     │
                     ▼
                 GPU Cluster

⸻

🌐 Final Vision

RIForge 最终并不只是：

一个 vLLM 部署脚本。

而是希望逐步形成：

                  RIForge
                     │
        ┌────────────┼────────────┐
        ▼            ▼            ▼
      TUI          API          Vue3
        │            │            │
        └────────────┼────────────┘
                     ▼
              Inference Control
                     │
        ┌────────────┼────────────┐
        ▼            ▼            ▼
     Models       Gateway      Observe
        │            │            │
        └────────────┼────────────┘
                     ▼
                Kubernetes
                     │
                     ▼
                 GPU Cluster
                     │
          ┌──────────┼──────────┐
          ▼          ▼          ▼
        vLLM       vLLM       vLLM

最终目标：

让模型部署像安装软件一样简单，让 GPU 推理服务像 Kubernetes Workload 一样可管理。

⸻

🤝 Contributing

欢迎提交：

* Issue
* Feature
* Pull Request
* Model Adapter
* Quantization Pipeline
* Deployment Backend
* Kubernetes Provider
* Observability Integration

⸻

📄 License

License: TBD

⸻

⭐ Project Status

RIForge is currently under active development.

当前重点：

✓ Model Download
✓ Model Quantization
✓ Docker Deployment
✓ vLLM Runtime
✓ TUI
→ Kubernetes
→ Gateway
→ Observability
→ AI Inference Platform

RIForge 的最终方向不是做一个更复杂的 docker run 包装器，而是构建一套围绕 LLM Deployment → Runtime → Gateway → Observability → Management 的自动化基础设施。