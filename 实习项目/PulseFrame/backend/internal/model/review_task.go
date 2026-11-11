package model

import "time"

// ReviewTask 状态取值：1待投递/2审核中/3通过/4驳回/5投递失败/6已取消。
const (
	ReviewTaskPendingDispatch uint8 = 1 // 待投递，等待 worker 领取并投递给审核服务。
	ReviewTaskReviewing       uint8 = 2 // 审核中，已投递，等待审核结论或对账。
	ReviewTaskPassed          uint8 = 3 // 通过，审核服务给出通过结论。
	ReviewTaskRejected        uint8 = 4 // 驳回，审核服务给出驳回结论。
	ReviewTaskDispatchFailed  uint8 = 5 // 投递失败，多次重试后仍未成功。
	ReviewTaskCancelled       uint8 = 6 // 已取消，作者撤回或删除后取消本轮投递。
)

// ReviewTask 表示一轮送审记录，兼作待投递队列。status 为 1 或 2 的行由 worker 领取处理。
type ReviewTask struct {
	// ID 为任务唯一标识，即投递给审核服务的 task_id。
	ID uint64 `gorm:"primaryKey;autoIncrement"`
	// VideoID 为关联视频标识。
	VideoID uint64 `gorm:"type:bigint unsigned;not null;uniqueIndex:uk_review_video_round,priority:1"`
	// ReviewRound 为对应视频的审核轮次，同一视频同一轮只有一条任务。
	ReviewRound uint32 `gorm:"type:int unsigned;not null;uniqueIndex:uk_review_video_round,priority:2"`
	// Status 表示任务状态，取值见包级常量。
	Status uint8 `gorm:"type:tinyint unsigned;not null;default:1;index:idx_review_due,priority:1"`
	// Reason 保存驳回原因，默认空串。
	Reason string `gorm:"type:varchar(200);not null;default:''"`
	// AttemptCount 保存领取次数，达到 5 次后置为投递失败。
	AttemptCount uint16 `gorm:"type:smallint unsigned;not null;default:0"`
	// NextAttemptAt 保存下次可被领取的时间：待投递时是重试时间，审核中时是对账时间。
	NextAttemptAt time.Time `gorm:"type:datetime(3);not null;index:idx_review_due,priority:2"`
	// CreatedAt 为提交时间，用于审核时长统计。
	CreatedAt time.Time `gorm:"type:datetime(3);not null"`
	// DecidedAt 为出结论时间，为空表示尚无结论。
	DecidedAt *time.Time `gorm:"type:datetime(3)"`
	// UpdatedAt 为最近修改时间，排查问题用。
	UpdatedAt time.Time `gorm:"type:datetime(3);not null"`
}

// TableName 返回送审任务表名。
// 输入：无。
// 输出：review_task。
// 功能：固定表名避免 gorm 复数化歧义。
// 例如该任务模型始终映射到 review_task 表。
func (ReviewTask) TableName() string {
	return "review_task"
}
