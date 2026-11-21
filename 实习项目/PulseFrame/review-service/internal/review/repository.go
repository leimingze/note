package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"pulseframe-review/internal"
)

var (
	ErrTaskNotFound   = errors.New("task not found")
	ErrTaskNotInHuman = errors.New("task is not waiting for human review")
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// CreateTask 创建审核任务；task_id 已存在时返回已有任务，created 为 false（重复投递返回 200）。
// 用唯一索引 + ON DUPLICATE 去重，并发重复投递也只会建一行。
func (r *Repository) CreateTask(ctx context.Context, req SubmitRequest, now time.Time) (*internal.Task, bool, error) {
	task := &internal.Task{
		TaskID: req.TaskID, Kind: req.Kind, VideoID: req.VideoID, ReviewRound: req.ReviewRound,
		MediaKey: req.MediaKey, CoverKey: req.CoverKey, Title: req.Title, Description: req.Description,
		Trigger: req.Trigger, Detail: req.Detail, Status: internal.TaskPending,
		NextAttemptAt: now, NextCallbackAt: now,
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(task)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		return task, true, nil
	}
	existing, err := r.GetTask(ctx, req.TaskID)
	return existing, false, err
}

// GetTask 根据 task_id 查询任务。
func (r *Repository) GetTask(ctx context.Context, taskID uint64) (*internal.Task, error) {
	var task internal.Task
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

// ClaimTask 领取一个到期的任务：待机审的任务，或租约已过期（进程崩溃）的机审中任务。
// 短事务内 SELECT ... FOR UPDATE SKIP LOCKED，再把 next_attempt_at 推后一个租约，与内容模块的投递 worker 同一做法；
// 多个 worker 并发领取时互相跳过被锁的行。没有可领的任务时返回 nil。
func (r *Repository) ClaimTask(ctx context.Context, now time.Time, lease time.Duration) (*internal.Task, error) {
	var claimed *internal.Task
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task internal.Task
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status IN ? AND next_attempt_at <= ?", []string{internal.TaskPending, internal.TaskRunning}, now).
			Order("next_attempt_at").Limit(1).Find(&task).Error
		if err != nil || task.ID == 0 {
			return err
		}
		task.Status = internal.TaskRunning
		task.Attempts++
		task.NextAttemptAt = now.Add(lease)
		claimed = &task
		return tx.Model(&internal.Task{}).Where("id = ?", task.ID).Updates(map[string]any{
			"status": task.Status, "attempts": task.Attempts, "next_attempt_at": task.NextAttemptAt,
		}).Error
	})
	return claimed, err
}

// RetryLater 机审故障后放回队列，next_attempt_at 之后再领取。
func (r *Repository) RetryLater(ctx context.Context, task *internal.Task, at time.Time, cause string) error {
	return r.db.WithContext(ctx).Model(&internal.Task{}).Where("id = ? AND status = ? AND attempts = ?", task.ID, internal.TaskRunning, task.Attempts).
		Updates(map[string]any{"status": internal.TaskPending, "next_attempt_at": at, "last_error": truncate(cause, 500)}).Error
}

// Outcome 是一次机审（或人审）得出的结论，写回任务表。
type Outcome struct {
	Status        string // completed 或 human_review
	Result        string // pass / limit / reject；转人审时为空
	Reason        string
	HumanReason   string
	Level         string
	PolicyVersion string
	FileHash      string
	ResultJSON    string
}

// SaveOutcome 写入结论。completed 的任务同时进入待回调状态；human_review 不回调，等人审结论。
// 只更新仍在机审中的任务，进程崩溃后被别的 worker 重新领取并已写过结论时不会覆盖。
func (r *Repository) SaveOutcome(ctx context.Context, task *internal.Task, outcome Outcome, now time.Time) error {
	updates := map[string]any{
		"status": outcome.Status, "result": outcome.Result, "reason": truncate(outcome.Reason, 500),
		"human_reason": truncate(outcome.HumanReason, 500), "level": outcome.Level,
		"policy_version": outcome.PolicyVersion, "file_hash": outcome.FileHash, "result_json": outcome.ResultJSON,
		"last_error": "",
	}
	if outcome.Status == internal.TaskCompleted {
		updates["decided_at"] = now
		updates["callback_state"] = internal.CallbackPending
		updates["next_callback_at"] = now
	}
	return r.db.WithContext(ctx).Model(&internal.Task{}).Where("id = ? AND status = ? AND attempts = ?", task.ID, internal.TaskRunning, task.Attempts).
		Updates(updates).Error
}

// SaveHumanDecision 写入人审结论并进入待回调；只接受正在等人审的任务。
func (r *Repository) SaveHumanDecision(ctx context.Context, taskID uint64, result, reason string, now time.Time) (*internal.Task, error) {
	res := r.db.WithContext(ctx).Model(&internal.Task{}).
		Where("task_id = ? AND status = ?", taskID, internal.TaskHumanReview).
		Updates(map[string]any{
			"status": internal.TaskCompleted, "result": result, "reason": truncate(reason, 500), "decided_at": now,
			"callback_state": internal.CallbackPending, "next_callback_at": now,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		if _, err := r.GetTask(ctx, taskID); err != nil {
			return nil, err
		}
		return nil, ErrTaskNotInHuman
	}
	return r.GetTask(ctx, taskID)
}

// DueCallbacks 返回到了回调时间的任务。
func (r *Repository) DueCallbacks(ctx context.Context, now time.Time, limit int) ([]internal.Task, error) {
	var tasks []internal.Task
	err := r.db.WithContext(ctx).Where("callback_state = ? AND next_callback_at <= ?", internal.CallbackPending, now).
		Order("next_callback_at").Limit(limit).Find(&tasks).Error
	return tasks, err
}

// MarkCallback 记录一次回调的结果：成功标记 done；失败累加次数并推后下次时间。
func (r *Repository) MarkCallback(ctx context.Context, id uint64, sendErr error, next time.Time, now time.Time) error {
	if sendErr == nil {
		return r.db.WithContext(ctx).Model(&internal.Task{}).Where("id = ?", id).
			Updates(map[string]any{"callback_state": internal.CallbackDone, "callback_at": now, "callback_error": ""}).Error
	}
	return r.db.WithContext(ctx).Model(&internal.Task{}).Where("id = ?", id).Updates(map[string]any{
		"callback_attempts": gorm.Expr("callback_attempts + 1"), "next_callback_at": next,
		"callback_error": truncate(sendErr.Error(), 500),
	}).Error
}

// FindFileRecord 查找文件在当前识别版本下的观察结果，没有时返回 nil。
func (r *Repository) FindFileRecord(ctx context.Context, fileHash, identVersion string) (*internal.FileRecord, error) {
	var record internal.FileRecord
	err := r.db.WithContext(ctx).Where("file_hash = ? AND ident_version = ?", fileHash, identVersion).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &record, err
}

// SaveFileRecord 保存机审观察结果；已有人工修正时不覆盖（人审结果优先，流程设计 2.1）。
func (r *Repository) SaveFileRecord(ctx context.Context, fileHash, identVersion, observationJSON string) error {
	record := internal.FileRecord{FileHash: fileHash, IdentVersion: identVersion, Done: true, ObservationJSON: observationJSON}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "file_hash"}, {Name: "ident_version"}},
		DoUpdates: clause.Assignments(map[string]any{
			"observation_json": gorm.Expr("IF(human_corrected, observation_json, VALUES(observation_json))"),
			"done":             true,
			"updated_at":       time.Now().UTC(),
		}),
	}).Create(&record).Error
}

// CorrectFileRecord 用人审修正后的观察结果覆盖机审结果，之后同一文件再上传时直接按修正结果处置。
func (r *Repository) CorrectFileRecord(ctx context.Context, fileHash, identVersion, observationJSON, reviewer string) error {
	return r.db.WithContext(ctx).Model(&internal.FileRecord{}).
		Where("file_hash = ? AND ident_version = ?", fileHash, identVersion).
		Updates(map[string]any{"observation_json": observationJSON, "human_corrected": true,
			"corrected_by": truncate(reviewer, 50), "done": true}).Error
}

// GetNextLimitSeq 原子地取视频的下一个限流序号（INSERT ... ON DUPLICATE KEY UPDATE seq = seq + 1）。
func (r *Repository) GetNextLimitSeq(ctx context.Context, videoID uint64) (uint64, error) {
	var seq uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := internal.LimitSeq{VideoID: videoID, Seq: 1}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "video_id"}},
			DoUpdates: clause.Assignments(map[string]any{"seq": gorm.Expr("seq + 1")}),
		}).Create(&row).Error; err != nil {
			return err
		}
		return tx.Model(&internal.LimitSeq{}).Where("video_id = ?", videoID).Select("seq").Scan(&seq).Error
	})
	return seq, err
}

// ComputeFileHash 从 reader 读取内容并计算 SHA-256。
func ComputeFileHash(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}
