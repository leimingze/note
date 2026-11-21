package internal

import "time"

// 任务状态。转人审期间不回调，内容对外保持"审核中"（流程设计 2.2、2.5）。
const (
	TaskPending     = "pending"      // 等待机审（含故障后等待重试）
	TaskRunning     = "running"      // 机审中，next_attempt_at 同时充当租约到期时间
	TaskHumanReview = "human_review" // 等待人审结论
	TaskCompleted   = "completed"    // 已有结论，回调按 callback_state 发送
)

// 回调状态。
const (
	CallbackNone    = ""        // 还没有结论
	CallbackPending = "pending" // 待发送或发送失败等待退避重试
	CallbackDone    = "done"    // 内容模块已返回 200
)

// Task 表示一个审核任务，task_id 由内容模块生成（review_task.id），两边共用。
type Task struct {
	ID               uint64     `gorm:"primaryKey;autoIncrement"`
	TaskID           uint64     `gorm:"uniqueIndex;not null"`         // 内容模块 review_task.id
	Kind             string     `gorm:"size:20;not null"`             // publish 发布审核 / rereview 复审
	VideoID          uint64     `gorm:"index;not null"`               // 关联的视频 ID
	ReviewRound      uint32     `gorm:"not null"`                     // 投递时的审核轮次，回调时原样带回
	MediaKey         string     `gorm:"size:255;not null"`            // 视频对象键
	CoverKey         string     `gorm:"size:255;not null"`            // 封面对象键
	Title            string     `gorm:"size:64;not null"`             // 视频标题
	Description      string     `gorm:"size:1000;not null"`           // 视频简介
	Trigger          string     `gorm:"size:20;not null;default:''"`  // 复审触发方式
	Detail           string     `gorm:"size:500;not null;default:''"` // 复审触发依据
	Status           string     `gorm:"size:20;not null;index:idx_claim,priority:1"`
	NextAttemptAt    time.Time  `gorm:"type:datetime(3);not null;index:idx_claim,priority:2"` // 下次可领取时间
	Attempts         int        `gorm:"not null;default:0"`                                   // 任务已领取次数，用于退避和识别当前领取者
	LastError        string     `gorm:"size:500;not null;default:''"`                         // 最近一次机审故障
	Result           string     `gorm:"size:20;not null;default:''"`                          // pass / limit / reject
	Reason           string     `gorm:"size:500;not null;default:''"`                         // 结论原因（回调里的 reason）
	HumanReason      string     `gorm:"size:500;not null;default:''"`                         // 转人审原因
	Level            string     `gorm:"size:4;not null;default:''"`                           // 低俗等级 L0–L3
	PolicyVersion    string     `gorm:"size:50;not null;default:''"`                          // 本次处置使用的规则表版本
	FileHash         string     `gorm:"index;size:64;not null;default:''"`                    // 原文件 SHA-256，关联 FileRecord
	ResultJSON       string     `gorm:"type:mediumtext"`                                      // 本次审核的完整结果，供人审回看和实验统计
	CallbackState    string     `gorm:"size:10;not null;default:'';index"`                    // 见 Callback* 常量
	CallbackAttempts int        `gorm:"not null;default:0"`                                   // 回调失败次数
	NextCallbackAt   time.Time  `gorm:"type:datetime(3);not null"`                            // 下次回调时间
	CallbackError    string     `gorm:"size:500;not null;default:''"`                         // 最近一次回调失败原因
	DecidedAt        *time.Time `gorm:"type:datetime(3)"`                                     // 得出结论的时间
	CallbackAt       *time.Time `gorm:"type:datetime(3)"`                                     // 回调成功的时间
	CreatedAt        time.Time  `gorm:"type:datetime(3);not null"`
	UpdatedAt        time.Time  `gorm:"type:datetime(3);not null"`
}

func (Task) TableName() string { return "task" }

// FileRecord 表示一个文件在某个识别版本下的观察结果（流程设计 2.1：文件摘要 + 识别版本唯一）。
// 只存观察结果，不存等级和处置：等级由程序按分级规则算，处置由当前规则表算，换表不用重跑模型。
type FileRecord struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement"`
	FileHash        string    `gorm:"uniqueIndex:idx_file_ident;size:64;not null"`  // SHA-256
	IdentVersion    string    `gorm:"uniqueIndex:idx_file_ident;size:100;not null"` // 识别版本 = 模型 + 提示词 + 观察项定义
	Done            bool      `gorm:"not null;default:false"`                       // 机审已完成，可复用
	ObservationJSON string    `gorm:"type:mediumtext"`                              // 观察结果 JSON（见 review.Observation）
	HumanCorrected  bool      `gorm:"not null;default:false"`                       // 是否经人工修正（优先于机审）
	CorrectedBy     string    `gorm:"size:50;not null;default:''"`                  // 修正人
	CreatedAt       time.Time `gorm:"type:datetime(3);not null"`
	UpdatedAt       time.Time `gorm:"type:datetime(3);not null"`
}

func (FileRecord) TableName() string { return "file_record" }

// LimitSeq 记录每个视频最新的限流推送序号，内容模块只接受更大的 seq，防止乱序。
type LimitSeq struct {
	VideoID   uint64    `gorm:"primaryKey"`
	Seq       uint64    `gorm:"not null;default:0"`
	UpdatedAt time.Time `gorm:"type:datetime(3);not null"`
}

func (LimitSeq) TableName() string { return "limit_seq" }

// Models 返回需要自动迁移的表。
func Models() []interface{} {
	return []interface{}{
		&Task{},
		&FileRecord{},
		&LimitSeq{},
	}
}
