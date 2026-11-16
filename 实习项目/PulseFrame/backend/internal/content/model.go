package model

import "time"

// Video 状态取值：0上传中/1草稿/2待审核/3已驳回/4已发布/5已隐藏/6已下架。
const (
	VideoStatusUploading uint8 = 0 // 上传中，仅作者可见，尚不可编辑。
	VideoStatusDraft     uint8 = 1 // 草稿，仅作者可见，可编辑与提交。
	VideoStatusReviewing uint8 = 2 // 待审核，不可编辑，等待审核结论。
	VideoStatusRejected  uint8 = 3 // 已驳回，作者可见驳回原因，可修改后重新提交。
	VideoStatusPublished uint8 = 4 // 已发布，对所有人可见。
	VideoStatusHidden    uint8 = 5 // 已隐藏，仅作者可见，可恢复公开。
	VideoStatusOffline   uint8 = 6 // 已下架，不可播放，作者不能自行恢复。
)

// Video 表示视频元数据表的一行记录。媒体文件本体存在对象存储，表内只存对象键与发布状态。
type Video struct {
	// ID 为视频唯一标识，自增主键，对象键也以它作为前缀。
	ID uint64 `gorm:"primaryKey;autoIncrement;index:idx_video_author_public,priority:4;index:idx_video_author_created,priority:4"`
	// AuthorID 为作者用户标识，与 users.id 一致，使用 ascii_bin 比较。
	AuthorID string `gorm:"type:char(36) character set ascii collate ascii_bin;not null;index:idx_video_author_public,priority:1;index:idx_video_author_created,priority:1"`
	// Title 保存视频标题，默认空串。
	Title string `gorm:"type:varchar(64);not null;default:''"`
	// Description 保存简介与话题文本，默认空串。
	Description string `gorm:"type:varchar(1000);not null;default:''"`
	// Status 表示当前发布状态，取值见包级常量。
	Status uint8 `gorm:"type:tinyint unsigned;not null;default:0;index:idx_video_author_public,priority:2;index:idx_video_created,priority:1"`
	// MediaKey 为正式视频对象键 videos/{id}/media.mp4，确认上传后写入，默认空串。
	MediaKey string `gorm:"type:varchar(255);not null;default:''"`
	// CoverKey 为正式封面对象键 videos/{id}/cover_{n}.{ext}，默认空串。
	CoverKey string `gorm:"type:varchar(255);not null;default:''"`
	// DurationMS 保存视频时长（毫秒），确认上传时由探测结果写入。
	DurationMS uint32 `gorm:"type:int unsigned;not null;default:0"`
	// Width 保存视频宽度（像素），用于客户端布局占位。
	Width uint16 `gorm:"type:smallint unsigned;not null;default:0"`
	// Height 保存视频高度（像素）。
	Height uint16 `gorm:"type:smallint unsigned;not null;default:0"`
	// SizeBytes 保存媒体文件大小（字节）。
	SizeBytes uint64 `gorm:"type:bigint unsigned;not null;default:0"`
	// ReviewRound 为当前审核轮次，每次提交加 1，回调据此判断新旧。
	ReviewRound uint32 `gorm:"type:int unsigned;not null;default:0"`
	// RejectReason 保存最近一次驳回原因，重新提交时清空。
	RejectReason string `gorm:"type:varchar(200);not null;default:''"`
	// OfflineReason 保存平台下架原因，作者可见，默认空串。
	OfflineReason string `gorm:"type:varchar(200);not null;default:''"`
	// PublishedAt 为首次审核通过时间，公开列表排序依据；为空表示尚未发布。
	PublishedAt *time.Time `gorm:"type:datetime(3);index:idx_video_author_public,priority:3"`
	// DeletedAt 为非空表示视频在回收站，用于回收站超期清理。
	DeletedAt *time.Time `gorm:"type:datetime(3);index:idx_video_author_created,priority:2;index:idx_video_cleanup,priority:1"`
	// CreatedAt 为视频创建时间，我的作品排序依据。
	CreatedAt time.Time `gorm:"type:datetime(3);not null;index:idx_video_author_created,priority:3;index:idx_video_created,priority:2"`
}

// TableName 返回视频表名。
// 输入：无。
// 输出：video。
// 功能：固定表名避免 gorm 复数化歧义。
// 例如该视频模型始终映射到 video 表。
func (Video) TableName() string {
	return "video"
}
