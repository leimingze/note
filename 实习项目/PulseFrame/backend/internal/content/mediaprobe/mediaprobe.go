package mediaprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ==================== 常量与配置 ====================

const (
	// maxProbeHeadBytes 是媒体探测时仅读取的对象头部大小，单位为字节。
	// moov 盒子位于文件头时，前几百 KB 即可拿到全部元数据，无需下载整个媒体文件。
	maxProbeHeadBytes = 512 * 1024
	// probeHTTPTimeout 是拉取对象头部时 HTTP 请求的总超时。
	probeHTTPTimeout = 30 * time.Second
)

// 媒体可接受的业务限制，与上传确认接口共用。
const (
	maxDurationMS uint32 = 600_000 // 视频时长上限（毫秒），对应设计文档中的 10 分钟。
	maxWidth             = 1920    // 视频宽度上限（像素），默认按 1080p 设定。
	maxHeight            = 1080    // 视频高度上限（像素），默认按 1080p 设定。
)

// 探测失败原因分类，供调用方给出具体拒绝原因。
var (
	ErrNoVideoStream         = errors.New("mediaprobe: no video stream")
	ErrHeadUnreadable        = errors.New("mediaprobe: cannot read media head (moov missing or corrupt)")
	ErrUnsupportedFormat     = errors.New("mediaprobe: unsupported container (only MP4)")
	ErrUnsupportedVideoCodec = errors.New("mediaprobe: video codec must be H.264")
	ErrUnsupportedAudioCodec = errors.New("mediaprobe: audio codec must be AAC")
	ErrTooLong               = errors.New("mediaprobe: duration exceeds limit")
	ErrResolutionOutOfRange  = errors.New("mediaprobe: resolution out of range")
)

// httpClient 仅用于拉取对象头部，保留为包级变量便于在测试中替换。
var httpClient = &http.Client{Timeout: probeHTTPTimeout}

// ==================== 类型、结构与数据模型 ====================

// Info 是一次成功探测得到的媒体摘要，供业务写入元数据并做合规判断。
type Info struct {
	Format     string // 容器格式名，来自 ffprobe 的 format_name。
	MajorBrand string // ftyp 盒子的 major_brand，用于区分 MP4 与 MOV。
	DurMS      uint32 // 视频时长（毫秒）。
	Width      int    // 视频宽度（像素）。
	Height     int    // 视频高度（像素）。
	VideoCodec string // 视频编码，如 h264。
	HasAudio   bool   // 是否含有音轨。
	AudioCodec string // 音轨编码，如 aac；无声视频为空串。
}

// ffprobeOutput 对应 ffprobe -of json 输出的顶层结构。
type ffprobeOutput struct {
	Streams []ffStream `json:"streams"`
	Format  ffFormat   `json:"format"`
}

// ffStream 对应 ffprobe 输出的单个流。
type ffStream struct {
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// ffFormat 对应 ffprobe 输出的容器格式信息。
type ffFormat struct {
	FormatName string            `json:"format_name"`
	Duration   string            `json:"duration"`
	Tags       map[string]string `json:"tags"`
}

// ==================== 解析与校验 ====================

// parseFFProbe 解析 ffprobe 的 JSON 输出为媒体摘要。
// 输入：raw，ffprobe -of json 的输出字节。
// 输出：Info；无视频流或 JSON 非法时返回哨兵错误。
// 功能：提取容器、主要编解码、分辨率与时长；热点只有音轨而无视频流时返回 ErrNoVideoStream。
func parseFFProbe(raw []byte) (*Info, error) {
	var o ffprobeOutput
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHeadUnreadable, err)
	}
	info := &Info{Format: o.Format.FormatName, MajorBrand: o.Format.Tags["major_brand"]}
	for i := range o.Streams {
		s := &o.Streams[i]
		switch s.CodecType {
		case "video":
			if info.VideoCodec == "" {
				info.VideoCodec = s.CodecName
				info.Width, info.Height = s.Width, s.Height
			}
		case "audio":
			info.HasAudio = true
			if info.AudioCodec == "" {
				info.AudioCodec = s.CodecName
			}
		}
	}
	if info.VideoCodec == "" {
		return nil, ErrNoVideoStream
	}
	if dur, err := strconv.ParseFloat(o.Format.Duration, 64); err == nil {
		info.DurMS = uint32(dur * 1000)
	}
	return info, nil
}

// Validate 检查媒体摘要是否符合可上架规则。
// 输入：info，探测得到的媒体摘要。
// 输出：合规返回 nil；不合规返回带原因的哨兵错误。
// 功能：应用容器、编码、时长与分辨率限制，供确认接口拒绝不合格媒体。
func Validate(info *Info) error {
	// ftyp 品牌为 4 字节字段，MOV 的品牌实际为 "qt  "（带填充空格），需去除空白后比较。
	if strings.TrimSpace(info.MajorBrand) == "qt" || !strings.Contains(info.Format, "mp4") {
		return ErrUnsupportedFormat
	}
	if info.VideoCodec != "h264" {
		return ErrUnsupportedVideoCodec
	}
	if info.HasAudio && info.AudioCodec != "aac" {
		return ErrUnsupportedAudioCodec
	}
	if info.DurMS == 0 || info.DurMS > maxDurationMS {
		return ErrTooLong
	}
	if info.Width == 0 || info.Height == 0 || info.Width > maxWidth || info.Height > maxHeight {
		return ErrResolutionOutOfRange
	}
	return nil
}

// ==================== 输入输出与外部服务 ====================

// fetchHead 仅拉取对象头部指定字节，避免下载整个媒体文件。
// 输入：ctx，上下文；url，对象签名地址；n，要读取的最大字节数。
// 输出：头部字节；请求失败时返回错误。
// 功能：发起带 Range 头的 GET，并截断响应体到 n 字节。
func fetchHead(ctx context.Context, url string, n int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", n-1))
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch head: status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, n))
}

// runFFProbe 通过标准输入把头部字节交给 ffprobe 解析。
// 输入：ctx，上下文；head，对象头部字节。
// 输出：ffprobe 的标准输出；ffprobe 不可用或无法解析时返回错误。
// 功能：执行 ffprobe 并返回其 JSON 输出。
func runFFProbe(ctx context.Context, head []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_format", "-show_streams", "-of", "json", "-i", "-")
	cmd.Stdin = bytes.NewReader(head)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHeadUnreadable, stderr.String())
	}
	return out, nil
}

// Probe 探测媒体对象并校验其是否符合上架规则。
// 输入：ctx，上下文；url，对象的短时签名地址。
// 输出：媒体摘要；任一步骤失败时返回哨兵错误。
// 功能：拉取头部、调用 ffprobe 解析并应用校验规则。
func Probe(ctx context.Context, url string) (*Info, error) {
	head, err := fetchHead(ctx, url, maxProbeHeadBytes)
	if err != nil {
		return nil, err
	}
	raw, err := runFFProbe(ctx, head)
	if err != nil {
		return nil, err
	}
	info, err := parseFFProbe(raw)
	if err != nil {
		return nil, err
	}
	if err := Validate(info); err != nil {
		return nil, err
	}
	return info, nil
}

// CheckAvailable 检查系统是否能调用 ffprobe。
// 输入：无。
// 输出：可用返回 nil；不可用返回错误。
// 功能：在 PATH 中查找 ffprobe，供服务启动时使用。
func CheckAvailable() error {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return fmt.Errorf("ffprobe not found: %w", err)
	}
	return nil
}
