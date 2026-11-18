package mediaprobe

import (
	"errors"
	"testing"
)

var sampleCases = []struct {
	name    string
	raw     string
	wantErr error
}{
	{
		name:    "h264 aac mp4 ok",
		raw:     `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1280,"height":720},{"index":1,"codec_type":"audio","codec_name":"aac"}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"120.000","tags":{"major_brand":"mp42"}}}`,
		wantErr: nil,
	},
	{
		name:    "silent h264 ok",
		raw:     `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"60.000","tags":{"major_brand":"isom"}}}`,
		wantErr: nil,
	},
	{
		name:    "h265 rejected",
		raw:     `{"streams":[{"index":0,"codec_type":"video","codec_name":"hevc","width":1920,"height":1080}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"60.000","tags":{"major_brand":"isom"}}}`,
		wantErr: ErrUnsupportedVideoCodec,
	},
	{
		name:    "mp3 audio rejected",
		raw:     `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1280,"height":720},{"index":1,"codec_type":"audio","codec_name":"mp3"}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"60.000","tags":{"major_brand":"mp42"}}}`,
		wantErr: ErrUnsupportedAudioCodec,
	},
	{
		name:    "mov rejected",
		raw:     `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"60.000","tags":{"major_brand":"qt  "}}}`,
		wantErr: ErrUnsupportedFormat,
	},
	{
		name:    "pure audio rejected",
		raw:     `{"streams":[{"index":0,"codec_type":"audio","codec_name":"aac"}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"60.000","tags":{"major_brand":"mp42"}}}`,
		wantErr: ErrNoVideoStream,
	},
	{
		name:    "too long rejected",
		raw:     `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1280,"height":720}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"700.000","tags":{"major_brand":"mp42"}}}`,
		wantErr: ErrTooLong,
	},
	{
		name:    "resolution out of range rejected",
		raw:     `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":3840,"height":2160}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"60.000","tags":{"major_brand":"mp42"}}}`,
		wantErr: ErrResolutionOutOfRange,
	},
	{
		name:    "invalid json rejected",
		raw:     `{"bad`,
		wantErr: ErrHeadUnreadable,
	},
}

// probeErr 运行完整解析与校验流程并返回最终错误。
// 输入：raw，ffprobe JSON 文本。
// 输出：解析或校验阶段返回的错误；全部通过时为 nil。
// 功能：封装解析与校验，简化测试断言。
func probeErr(raw string) error {
	info, err := parseFFProbe([]byte(raw))
	if err != nil {
		return err
	}
	return Validate(info)
}

// TestParseValidateSamples 验证解析与校验规则的分支行为。
// 输入：t，测试上下文。
// 输出：断言失败时测试失败。
// 功能：对代表性 ffprobe 输出断言通过或特定的拒绝原因。
func TestParseValidateSamples(t *testing.T) {
	for _, c := range sampleCases {
		t.Run(c.name, func(t *testing.T) {
			if err := probeErr(c.raw); !errors.Is(err, c.wantErr) {
				t.Fatalf("probeErr = %v, want %v", err, c.wantErr)
			}
		})
	}
}
