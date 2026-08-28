#!/usr/bin/env python3

import argparse
import json
import os
import sys

# 完成标记文件名，必须跟 Go 侧 internal/pipeline/quantize.go 里的
# CompletionMarker 常量保持一致，两边任何一处改动都要同步改另一处。
COMPLETION_MARKER = ".igforge_complete"
 
 
def normalize_tokenizer_config(output_path: str) -> None:
   
    config_path = os.path.join(output_path, "tokenizer_config.json")
    if not os.path.isfile(config_path):
        print(f"[quantize_awq] 警告: 没找到 {config_path}，跳过格式归一化")
        return
 
    with open(config_path, "r", encoding="utf-8") as f:
        data = json.load(f)
 
    extra = data.get("extra_special_tokens")
    if isinstance(extra, list):
        print(
            f"[quantize_awq] 检测到 extra_special_tokens 是旧的列表格式"
            f"（{len(extra)} 项），转换成字典格式以兼容 vLLM 的 transformers 版本"
        )
        data["extra_special_tokens"] = {token: token for token in extra}
        with open(config_path, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
    else:
        print("[quantize_awq] extra_special_tokens 格式正常，无需转换")

def parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="AutoAWQ 量化脚本")
    p.add_argument("--model-path", required=True, help="原始模型目录（bf16/fp16）")
    p.add_argument("--output-path", required=True, help="量化产物输出目录")
    p.add_argument("--w-bit", type=int, default=4, help="权重量化位宽，默认 4")
    p.add_argument("--q-group-size", type=int, default=128, help="量化分组大小，默认 128")
    p.add_argument(
        "--zero-point",
        action="store_true",
        default=True,
        help="是否使用 zero-point 量化（AutoAWQ 默认推荐开启）",
    )
    p.add_argument(
        "--calib-data",
        default=None,
        help="自定义校准数据集路径（jsonl，每行一个 {\"text\": ...}）；"
        "不传则使用 AutoAWQ 内置的 pileval 数据集",
    )
    return p.parse_args()


def main() -> int:
    args = parse_args()

    if not os.path.isdir(args.model_path):
        print(f"[quantize_awq] 错误: 输入目录不存在: {args.model_path}", file=sys.stderr)
        return 1

    os.makedirs(args.output_path, exist_ok=True)

    try:
        from awq import AutoAWQForCausalLM
        from transformers import AutoTokenizer
    except ImportError as e:
        print(
            "[quantize_awq] 错误: 缺少依赖，请先 pip install autoawq\n"
            f"原始错误: {e}",
            file=sys.stderr,
        )
        return 1

    quant_config = {
        "zero_point": args.zero_point,
        "q_group_size": args.q_group_size,
        "w_bit": args.w_bit,
        "version": "GEMM",  # GEMM 版本对 vLLM 的兼容性最好，跑不动再考虑 GEMV
    }

    print(f"[quantize_awq] 加载模型: {args.model_path}")
    model = AutoAWQForCausalLM.from_pretrained(
        args.model_path,
        safetensors=True,
        device_map="auto",
    )
    tokenizer = AutoTokenizer.from_pretrained(args.model_path, trust_remote_code=True)

    quantize_kwargs = {"quant_config": quant_config, "tokenizer": tokenizer}
    if args.calib_data:
        print(f"[quantize_awq] 使用自定义校准数据集: {args.calib_data}")
        import json

        texts = []
        with open(args.calib_data, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                texts.append(json.loads(line)["text"])
        quantize_kwargs["calib_data"] = texts
    else:
        print("[quantize_awq] 使用 AutoAWQ 内置校准数据集 (pileval)")

    print(
        f"[quantize_awq] 开始量化: w_bit={args.w_bit} "
        f"q_group_size={args.q_group_size} version=GEMM"
    )
    model.quantize(**quantize_kwargs)

    print(f"[quantize_awq] 保存量化产物到: {args.output_path}")
    model.save_quantized(args.output_path)
    tokenizer.save_pretrained(args.output_path)

    normalize_tokenizer_config(args.output_path)

    marker_path = os.path.join(args.output_path, COMPLETION_MARKER)
    with open(marker_path, "w", encoding="utf-8") as f:
        f.write("ok\n")

    print("[quantize_awq] 完成")
    return 0


if __name__ == "__main__":
    sys.exit(main())
