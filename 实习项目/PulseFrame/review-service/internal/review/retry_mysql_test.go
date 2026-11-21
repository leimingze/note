//go:build mysqlintegration

package review

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"pulseframe-review/internal"
	"pulseframe-review/internal/platform/config"
)

// 输入：专用空测试库 REVIEW_TEST_MYSQL_DSN。
// 输出：创建任务并核验持续故障、租约接手和恢复后的结论。
// 功能：连续六次访问缺少可执行文件的探测服务，确认只有当前领取者能结束任务。
func TestQueueSurvivesServiceFailure(t *testing.T) {
	db, err := gorm.Open(mysql.Open(os.Getenv("REVIEW_TEST_MYSQL_DSN")))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&internal.Task{}); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	cfg, err := config.LoadReview()
	if err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(&Service{cfg: cfg}, repo)
	ctx := context.Background()
	now := time.Now().UTC()
	id := uint64(now.UnixNano())
	_, _, err = repo.CreateTask(ctx, SubmitRequest{TaskID: id}, now)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir()) // 让真实进程启动返回找不到 ffprobe。
	for attempt := 1; attempt <= 6; attempt++ {
		task, err := repo.ClaimTask(ctx, now.Add(time.Hour), cfg.Lease)
		if err != nil || task == nil || task.TaskID != id {
			t.Fatalf("claim: %v %v", task, err)
		}
		_, probeErr := worker.service.probeMedia(ctx, "video.mp4")
		if probeErr == nil {
			t.Fatal("expected unavailable ffprobe")
		}
		started := time.Now().UTC()
		worker.handleFailure(task, probeErr)
		row, err := repo.GetTask(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		delay := min(cfg.RetryBase<<(attempt-1), cfg.RetryMax)
		if row.Status != internal.TaskPending || row.Attempts != attempt || row.LastError == "" ||
			row.CallbackState != internal.CallbackNone || row.NextAttemptAt.Before(started.Add(delay-time.Millisecond)) ||
			row.NextAttemptAt.After(time.Now().UTC().Add(delay+time.Millisecond)) {
			t.Fatalf("unexpected retry: %+v", row)
		}
	}
	old, err := repo.ClaimTask(ctx, now.Add(time.Hour), cfg.Lease)
	if err != nil || old == nil {
		t.Fatalf("claim old: %v", err)
	}
	current, err := repo.ClaimTask(ctx, old.NextAttemptAt.Add(time.Second), cfg.Lease)
	if err != nil || current == nil {
		t.Fatalf("reclaim: %v", err)
	}
	worker.handleFailure(old, context.DeadlineExceeded)
	if err := repo.SaveOutcome(ctx, old, Outcome{Status: internal.TaskHumanReview}, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveOutcome(ctx, current, Outcome{Status: internal.TaskCompleted, Result: "pass"}, now); err != nil {
		t.Fatal(err)
	}
	row, err := repo.GetTask(ctx, id)
	if err != nil || row.Status != internal.TaskCompleted || row.Result != "pass" || row.CallbackState != internal.CallbackPending {
		t.Fatalf("resume: %+v %v", row, err)
	}
}
