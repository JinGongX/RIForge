// Package pipeline 把"下载 -> 量化 -> 部署"实现成三个独立的、可幂等重跑的阶段。
package pipeline

// Status 表示某个阶段的执行结果。
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusSkipped // 幂等检查发现已经完成，本次跳过
	StatusSuccess
	StatusFailed
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "等待中"
	case StatusRunning:
		return "执行中"
	case StatusSkipped:
		return "已跳过(已存在)"
	case StatusSuccess:
		return "成功"
	case StatusFailed:
		return "失败"
	default:
		return "未知"
	}
}

// StageName 标识流水线里的三个固定阶段。
type StageName string

const (
	StageDownload StageName = "下载模型"
	StageQuantize StageName = "量化模型"
	StageDeploy   StageName = "Docker 部署"
)

// StageResult 是某个阶段跑完之后的结果汇总，供 TUI 渲染。
type StageResult struct {
	Name   StageName
	Status Status
	Err    error
}
