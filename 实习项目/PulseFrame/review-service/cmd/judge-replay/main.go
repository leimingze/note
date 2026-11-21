// judge-replay 使用整片复核的观察项提示词和单次回答校验回放样本。
// 用于诊断实验，例如把挑中的插入帧单独送审，对比放在整批帧里时的认出率。
//
// 输入 JSONL 每行：{"sample_id": "...", "title": "", "ocr": "", "asr": "",
//
//	"frames": [{"path": "...", "timestamp": 7.6, "type": "brief_insertion", "crop_path": ""}]}
//
// 输出 JSONL 每行包含样本编号、观察结果及错误信息。
//
// 用法：VLM_BASE_URL=... VLM_MODEL=... judge-replay --input samples.jsonl --output decisions.jsonl \
//
//	--prompt policies/observation/system_prompt.txt --version qwen3.5-9b+obs-prompt-v1 --concurrency 4
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"

	"pulseframe-review/internal/judge"
)

type inputFrame struct {
	Path      string  `json:"path"`
	Timestamp float64 `json:"timestamp"`
	Type      string  `json:"type"`
	CropPath  string  `json:"crop_path"`
}

type input struct {
	SampleID string       `json:"sample_id"`
	Title    string       `json:"title"`
	OCR      string       `json:"ocr"`
	ASR      string       `json:"asr"`
	Frames   []inputFrame `json:"frames"`
}

type output struct {
	SampleID string         `json:"sample_id"`
	Decision judge.Decision `json:"decision"`
	Error    string         `json:"error,omitempty"`
}

func main() {
	inputPath := flag.String("input", "", "样本 JSONL")
	outputPath := flag.String("output", "", "判定结果 JSONL")
	promptPath := flag.String("prompt", "policies/observation/system_prompt.txt", "系统提示词")
	version := flag.String("version", "qwen3.5-9b+obs-prompt-v1", "识别版本")
	concurrency := flag.Int("concurrency", 4, "同时回放的样本数")
	flag.Parse()
	if *inputPath == "" || *outputPath == "" {
		log.Fatal("--input and --output are required")
	}
	prompt, err := os.ReadFile(*promptPath)
	if err != nil {
		log.Fatalf("read prompt: %v", err)
	}
	cfg, err := judge.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("load VLM config: %v", err)
	}
	engine := judge.Engine{Inference: judge.NewClientFromConfig(cfg)}
	policy := judge.Policy{Version: *version, AllowedLabels: judge.AllowedLabels, SystemPrompt: string(prompt)}

	samples, err := readInputs(*inputPath)
	if err != nil {
		log.Fatalf("read input: %v", err)
	}
	out, err := os.Create(*outputPath)
	if err != nil {
		log.Fatalf("create output: %v", err)
	}
	defer out.Close()

	var mu sync.Mutex
	var wg sync.WaitGroup
	queue := make(chan input)
	done := 0
	for range *concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for in := range queue {
				result := output{SampleID: in.SampleID}
				decision, err := engine.Judge(context.Background(), toSample(in), policy)
				if err != nil {
					result.Error = err.Error()
				} else {
					result.Decision = decision
				}
				line, _ := json.Marshal(result)
				mu.Lock()
				out.Write(append(line, '\n'))
				done++
				if done%20 == 0 {
					fmt.Fprintf(os.Stderr, "%d/%d\n", done, len(samples))
				}
				mu.Unlock()
			}
		}()
	}
	for _, in := range samples {
		queue <- in
	}
	close(queue)
	wg.Wait()
	fmt.Fprintf(os.Stderr, "done: %d samples\n", len(samples))
}

func readInputs(path string) ([]input, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var samples []input
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<24)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var in input
		if err := json.Unmarshal(scanner.Bytes(), &in); err != nil {
			return nil, fmt.Errorf("parse line %d: %w", len(samples)+1, err)
		}
		samples = append(samples, in)
	}
	return samples, scanner.Err()
}

func toSample(in input) judge.Sample {
	frames := make([]judge.Frame, 0, len(in.Frames))
	for _, f := range in.Frames {
		frames = append(frames, judge.Frame{Path: f.Path, Timestamp: f.Timestamp, Type: f.Type, CropPath: f.CropPath})
	}
	return judge.Sample{SampleID: in.SampleID, Title: in.Title, OCR: in.OCR, ASR: in.ASR, Frames: frames}
}
