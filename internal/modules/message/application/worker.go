// worker.go：异步发送 worker——sendLoop/attempt 状态机面。
// 拆分自 service.go（R7 红线 #14：单文件 400 行警告线）。
package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/netyilei/netyadmin/internal/modules/message/domain"
)

// sendLoop 是发送 worker（goroutine 惯例：named loop+panic 隔离；单 worker
// 保序——SMTP 串行投递）。recover 在循环内：单次 panic 不杀 worker（联测
// 审计 D7——recover 在循环外=panic 后 worker 永久退出）。
func (s *Service) sendLoop() {
	defer s.wg.Done()
	for {
		select {
		case <-s.stop:
			return
		case rec := <-s.queue:
			s.metrics.SetQueueDepth(len(s.queue))
			s.safeAttempt(rec)
		}
	}
}

// safeAttempt 单次发送的 panic 隔离边界。
func (s *Service) safeAttempt(rec *domain.SendRecord) {
	defer func() {
		if r := recover(); r != nil {
			s.logger().Error("message: send attempt panic", "record", rec.ID, "panic", fmt.Sprint(r))
		}
	}()
	s.attempt(rec)
}

// attempt 执行一次发送（claim CAS 抢占 → 驱动调用 → 状态回写）。
// 修正：worker 与 RetryJob 并发对同一记录 attempt=双投——先 CAS 抢占
// （retries 期望值匹配），抢不到=他方在发，直接返回。
func (s *Service) attempt(rec *domain.SendRecord) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	claimed, err := s.repo.Records.Claim(ctx, rec.ID, rec.Retries)
	if err != nil {
		s.logUpdateErr(rec.ID, "claim", err)
		return
	}
	if !claimed {
		return // 他方已抢占（并发 attempt 语义）
	}
	rec.Retries++ // claim 已推进
	d, err := s.registry.Active(rec.Channel)
	if err != nil {
		s.metrics.IncSendFailed(string(rec.Channel))
		if uerr := s.repo.Records.UpdateStatus(ctx, rec.ID, domain.RecordFailed, rec.Retries, "no active driver"); uerr != nil {
			s.logUpdateErr(rec.ID, "status(no-driver)", uerr)
		}
		return
	}
	err = d.Send(ctx, domain.Outbound{To: rec.Target, Subject: rec.Subject, Body: rec.Body})
	if err == nil {
		now := s.now()
		if uerr := s.repo.Records.UpdateStatus(ctx, rec.ID, domain.RecordSent, rec.Retries, ""); uerr != nil {
			s.logUpdateErr(rec.ID, "status(sent)", uerr)
		}
		rec.Status, rec.SentAt = domain.RecordSent, &now
		return
	}
	status := domain.RecordFailed
	if rec.Retries < domain.MaxRetries {
		status = domain.RecordPending // retry Job 重投
	}
	s.metrics.IncSendFailed(string(rec.Channel))
	if uerr := s.repo.Records.UpdateStatus(ctx, rec.ID, status, rec.Retries, err.Error()); uerr != nil {
		s.logUpdateErr(rec.ID, "status(failed)", uerr)
	}
	if s.errorLog != nil {
		s.errorLog.Error("message: send failed", "record", rec.ID, "channel", rec.Channel, "err", err)
	}
}

// logUpdateErr 状态回写失败告警（reliability：禁静默吞——记录恒 pending 由
// retry Job 重扫，但需可观测）。
func (s *Service) logUpdateErr(recordID, op string, err error) {
	if s.errorLog != nil {
		s.errorLog.Error("message: record status update failed", "record", recordID, "op", op, "err", err)
	}
}

func (s *Service) logger() *slog.Logger {
	if s.accessLog != nil {
		return s.accessLog
	}
	return slog.Default()
}
