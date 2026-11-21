package review

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"pulseframe-review/internal"
)

// Worker 从任务表领取任务做机审，并负责回调重试。任务表就是队列：投递只插一行，
// worker 用 SKIP LOCKED 领取，不需要额外的消息队列或 Redis。
type Worker struct {
	service *Service
	repo    *Repository
}

func NewWorker(service *Service, repo *Repository) *Worker {
	return &Worker{service: service, repo: repo}
}

// Run 启动 n 个机审协程和 1 个回调协程，ctx 取消后等当前任务做完再返回。
func (w *Worker) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for index := range w.service.cfg.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.reviewLoop(ctx, index)
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.callbackLoop(ctx)
	}()
	wg.Wait()
}

func (w *Worker) reviewLoop(ctx context.Context, index int) {
	cfg := w.service.cfg
	for ctx.Err() == nil {
		task, err := w.repo.ClaimTask(ctx, time.Now().UTC(), cfg.Lease)
		if err != nil {
			slog.Error("claim task failed", "worker", index, "error", err)
		}
		if task == nil {
			sleep(ctx, cfg.PollInterval)
			continue
		}
		// 任务处理不随服务退出中断：用独立的 context，避免写了一半的结论。
		runErr := w.service.ProcessTask(context.WithoutCancel(ctx), task)
		if runErr == nil {
			continue
		}
		w.handleFailure(task, runErr)
	}
}

// handleFailure 机审故障：间隔 1、2、4 分钟重试，最多 3 次；仍失败转人审，内容保持不公开（附录 F）。
func (w *Worker) handleFailure(task *internal.Task, runErr error) {
	ctx := context.Background()
	cfg := w.service.cfg
	if task.Attempts >= cfg.MaxAttempts {
		reason := fmt.Sprintf("机审 %d 次均失败：%v", task.Attempts, runErr)
		err := w.repo.SaveOutcome(ctx, task.ID, Outcome{Status: internal.TaskHumanReview, HumanReason: reason}, time.Now().UTC())
		slog.Error("task moved to human review after retries", "task_id", task.TaskID, "error", runErr, "save_error", err)
		return
	}
	next := time.Now().UTC().Add(cfg.RetryBase << (task.Attempts - 1))
	err := w.repo.RetryLater(ctx, task.ID, next, runErr.Error())
	slog.Warn("task failed, will retry", "task_id", task.TaskID, "attempt", task.Attempts, "next", next,
		"error", runErr, "save_error", err)
}

// callbackLoop 把有结论的任务回调给内容模块；失败按退避重试直到成功（1s、2s、4s……最长 5 分钟）。
func (w *Worker) callbackLoop(ctx context.Context) {
	cfg := w.service.cfg
	for ctx.Err() == nil {
		tasks, err := w.repo.DueCallbacks(ctx, time.Now().UTC(), 20)
		if err != nil {
			slog.Error("load callbacks failed", "error", err)
		}
		for _, task := range tasks {
			sendErr := w.service.content.Callback(ctx, CallbackRequest{TaskID: task.TaskID, Result: task.Result, Reason: task.Reason})
			delay := cfg.CallbackBase << min(task.CallbackAttempts, 20)
			if delay > cfg.CallbackMax || delay <= 0 {
				delay = cfg.CallbackMax
			}
			now := time.Now().UTC()
			if err := w.repo.MarkCallback(ctx, task.ID, sendErr, now.Add(delay), now); err != nil {
				slog.Error("mark callback failed", "task_id", task.TaskID, "error", err)
			}
			if sendErr != nil {
				slog.Warn("callback failed, will retry", "task_id", task.TaskID, "attempt", task.CallbackAttempts+1,
					"retry_in", delay, "error", sendErr)
			} else {
				slog.Info("callback sent", "task_id", task.TaskID, "result", task.Result)
			}
		}
		if len(tasks) == 0 {
			sleep(ctx, cfg.PollInterval)
		}
	}
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
