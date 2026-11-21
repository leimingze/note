package review

import (
	"pulseframe-review/internal/judge"
	"pulseframe-review/internal/platform/config"
	"pulseframe-review/internal/policy"
	"slices"
	"testing"
)

// 输入：两个时间窗口以及跨窗口的语音和画面。
// 输出：检查批次文字、前后帧和全片证据定位。
// 功能：在批界两侧都命中时，重复的相邻帧只送一次，原话和全片时间可追溯。
func TestCrossBatchEvidence(t *testing.T) {
	text := TextResult{ASRSegments: []TranscriptLine{{Start: 1, End: 4, Text: "请勿模仿"}}, OCR: []TranscriptLine{{Timestamp: 4, Text: "展示说明"}}}
	_, sources, refs := batchText(&text, 0, 3)
	if !refs["ASR1"] || refs["OCR1"] {
		t.Fatalf("unexpected first batch references %v", refs)
	}
	_, later, refs := batchText(&text, 3, 6)
	if !refs["ASR1"] || !refs["OCR1"] {
		t.Fatalf("unexpected second batch references %v", refs)
	}
	first := BatchScreen{Sources: sources}
	first.Result.Value = []judge.Finding{{Frame: 2, References: []string{"ASR1"}}}
	second := BatchScreen{First: 2, Sources: later}
	second.Result.Value = []judge.Finding{{Frame: 1, References: []string{"OCR1"}}}
	all := []FrameInfo{{Timestamp: 0}, {Timestamp: 2}, {Timestamp: 4}, {Timestamp: 6}, {Timestamp: 8}}
	screens := []BatchScreen{first, second}
	if got := contextFrames(all, screens); !slices.Equal(got, all[:4]) {
		t.Fatalf("context frames: %v", got)
	}
	if clean := contextFrames(all, nil); len(clean) != 0 {
		t.Fatal("unexpected strong review frames")
	}
	hints := screenHints(all, screens)
	if hints[0].Related["ASR1"].Text != "请勿模仿" || hints[1].Timestamp != 4 || hints[1].Related["OCR1"].Timestamp != 4 {
		t.Fatalf("bad full-video evidence %v", hints)
	}
	if v := Decide(Observation{InvalidAnswer: "格式无效"}, nil); v.Action != policy.ActionHuman {
		t.Fatal(v)
	}
}

// 输入：抽样比例、相邻窗口的曝光增量和同一任务的重复提交。
// 输出：验证窗口门槛、零基数及重复选择。
// 功能：曝光从零增加到一百时可入选，从一增加到三时仍低于增量门槛。
func TestPublishedSampling(t *testing.T) {
	pct := 100.0
	cfg := config.Sampling{Mode: "exposure", Percent: &pct, WindowSeconds: 3600, MinimumIncrease: 100, GrowthFactor: 2}
	for _, tc := range []struct {
		prev, cur int64
		want      bool
	}{{0, 100, true}, {1, 3, false}, {200, 300, false}, {200, 400, true}, {400, 200, false}} {
		c := PublishedCandidate{WindowSeconds: 3600, Previous: &tc.prev, Current: &tc.cur}
		got, err := selectedPublished(cfg, c)
		if err != nil || got != tc.want {
			t.Fatalf("%v to %v: %v %v", tc.prev, tc.cur, got, err)
		}
	}
	c := PublishedCandidate{SubmitRequest: SubmitRequest{TaskID: 123, VideoID: 8}}
	if _, err := selectedPublished(cfg, c); err == nil {
		t.Fatal("missing exposure data accepted")
	}
	cfg.Mode = "random"
	for _, percent := range []float64{0, 5, 100} {
		pct = percent
		got, err := selectedPublished(cfg, c)
		again, _ := selectedPublished(cfg, c)
		if err != nil || got != again || (pct == 0 && got) || (pct == 100 && !got) {
			t.Fatal("unstable selection", pct, got, again, err)
		}
	}
}
