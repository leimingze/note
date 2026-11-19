# 脚本功能：使用 KuaiMod 附录 A.1.3 的两项分类任务，保存模型原文和官方评分标签。
# 启动命令（从 backend 运行）：
# python tools/replay/evaluate_paper.py --sample-id 0000 --output-dir reports/paper-0000

import argparse
import base64
import json
import mimetypes
from pathlib import Path

from openai import OpenAI

# 模板按论文图 A-1(c)、(d) 转录，图中的回答形式作为最后一句提示。
PROMPTS = Path(__file__).resolve().parents[2] / "policies" / "kuaimod_paper"
# 官方 multi_cls_eval.py 的完整分类表，所有样本使用相同的候选类别。
LABELS = ("正向", "违法犯罪", "涉政不当言行", "辱骂、引战、宣扬仇恨", "出售违禁品",
          "历史虚无主义", "色情低俗", "不实信息", "危害未成年人身心健康", "未成年不适当行为",
          "夸大商品功效", "引导私下交易", "虚假承诺福利", "炒作卖货", "未添加AI标识", "盗用抄袭作品")
# 英文问题问是否违规，官方评分脚本用中文“否”编码违规。
BINARY_LABELS = {"yes": "否", "no": "是"}


def make_message(sample: dict, task: str) -> list[dict]:
    """
    输入：sample 为项目样本，task 为 binary 或 multiclass。
    输出：包含图像和文字的一条用户消息，读取本地图片和提示词文件。
    功能：按论文顺序组合专家角色、图像、元数据和问题，例如公开集的评论字段留空。
    """
    metadata = {key: sample[key] for key in ("title", "ocr", "asr")}
    for key in ("description", "comments"):
        metadata[key] = sample.get(key, "")
    context = (PROMPTS / "context.txt").read_text(encoding="utf-8").strip().format(**metadata)
    question = (PROMPTS / f"{task}.txt").read_text(encoding="utf-8").strip()
    content = [{"type": "text", "text": "You are a video moderation expert."}]
    for filename in sample["images"]:
        path = Path(filename)
        mime, _ = mimetypes.guess_type(filename)
        if mime not in {"image/jpeg", "image/png", "image/webp"}:
            raise ValueError(f"Unsupported image type: {path}")
        encoded = base64.b64encode(path.read_bytes()).decode("ascii")
        content.append({"type": "image_url", "image_url": {"url": f"data:{mime};base64,{encoded}"}})
    content.append({"type": "text", "text": context + "\n" + question.format(taxonomy="、".join(LABELS))})
    return [{"role": "user", "content": content}]


def classify(client: OpenAI, sample: dict, *, task: str, model: str) -> dict:
    """
    输入：本地模型客户端、单条样本、任务名和模型名。
    输出：一次模型请求的原文、评分标签及正确性，无法识别的输出记为无效。
    功能：二分类回答 Yes 时保存原文并按官方约定记录“否”，多分类直接核对类别。
    """
    response = client.chat.completions.create(
        model=model, messages=make_message(sample, task), temperature=0, n=1,
    )
    raw = response.choices[0].message.content or ""
    answer = raw.strip().rstrip(".。").strip()
    tag = sample["tag"]
    expected = tag
    judgement = answer if answer in LABELS else None
    if task == "binary":
        judgement = BINARY_LABELS.get(answer.casefold())
        expected = "是" if tag == "正向" else "否"
    return {"sample_id": sample["sample_id"], "tag": tag, "task": task, "model": model,
            "raw": raw, "judgement": judgement, "valid": judgement is not None,
            "correct": judgement == expected}


def main() -> None:
    """
    输入：样本清单、明确指定的样本编号、新输出目录及模型连接参数。
    输出：访问推理接口，在新目录分别生成 binary.jsonl 和 multiclass.jsonl。
    功能：对选定样本分别运行两项任务，例如一个编号产生两次独立判断。
    """
    parser = argparse.ArgumentParser(description="KuaiMod 附录 A.1.3 分类评测")
    parser.add_argument("--samples", type=Path, default=Path("data/kuaimod/samples.jsonl"))
    parser.add_argument("--sample-id", action="append", required=True)
    parser.add_argument("--output-dir", required=True, type=Path)
    parser.add_argument("--base-url", default="http://127.0.0.1:8000/v1")
    parser.add_argument("--model", default="Qwen/Qwen3.5-9B")  # 默认使用服务器上的 9B 多模态模型。
    args = parser.parse_args()
    with args.samples.open(encoding="utf-8") as source:
        by_id = {row["sample_id"]: row for row in map(json.loads, filter(str.strip, source))}
    selected = [by_id[key] for key in dict.fromkeys(args.sample_id)]
    client = OpenAI(base_url=args.base_url, api_key="local", max_retries=0)
    args.output_dir.mkdir(parents=True, exist_ok=False)
    for task in ("binary", "multiclass"):
        with (args.output_dir / f"{task}.jsonl").open("x", encoding="utf-8") as output:
            for sample in selected:
                row = classify(client, sample, task=task, model=args.model)
                print(json.dumps(row, ensure_ascii=False), file=output, flush=True)


if __name__ == "__main__":
    main()
