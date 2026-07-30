package message

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	msgEntity "NetyAdmin/internal/domain/entity/message"
	"NetyAdmin/internal/pkg/configsync"
	"NetyAdmin/internal/pkg/database"
	"NetyAdmin/internal/pkg/errorx"
	msgPkg "NetyAdmin/internal/pkg/message"
	"NetyAdmin/internal/pkg/task"
	msgRepo "NetyAdmin/internal/repository/message"
)

type MsgSendJob struct {
	repo    msgRepo.MsgRepository
	drivers map[string]msgPkg.Driver
	watcher configsync.ConfigWatcher
	tm      database.TxManager
}

func NewMsgSendJob(repo msgRepo.MsgRepository, drivers map[string]msgPkg.Driver, watcher configsync.ConfigWatcher, tm database.TxManager) *MsgSendJob {
	return &MsgSendJob{
		repo:    repo,
		drivers: drivers,
		watcher: watcher,
		tm:      tm,
	}
}

func (j *MsgSendJob) Name() string {
	return "msg_send_job"
}

func (j *MsgSendJob) DisplayName() string {
	return "消息发送异步任务"
}

func (j *MsgSendJob) DefaultMetadata() task.TaskMetadata {
	return task.TaskMetadata{
		Name:        j.Name(),
		DisplayName: j.DisplayName(),
		Type:        task.TypeOnce,
		Enabled:     true,
		Weight:      task.WeightEssential,
	}
}

func (j *MsgSendJob) Run(ctx context.Context) error {
	return nil
}

func (j *MsgSendJob) isChannelEnabled(channel string) bool {
	var val string
	var exists bool
	switch channel {
	case "email":
		val, exists = j.watcher.GetConfig("email_config", "enabled")
	case "sms":
		val, exists = j.watcher.GetConfig("sms_config", "enabled")
	default:
		return true
	}
	if !exists {
		return false
	}
	return val == "true" || val == "1"
}

func (j *MsgSendJob) Execute(ctx context.Context, payload json.RawMessage) error {
	var recordID uint64
	if err := json.Unmarshal(payload, &recordID); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	rec, err := j.repo.GetRecordByID(ctx, recordID)
	if err != nil {
		return fmt.Errorf("record not found: %w", err)
	}

	if rec.Status != msgEntity.MsgStatusPending {
		return nil
	}

	if rec.Channel == "internal" {
		// TM 单事务原子完成「更新投递记录状态 + 创建内部信箱消息」，任一步失败整体回滚（fail-closed）。
		// 消除「假成功」反模式：避免状态已置 Success 但信箱未投递。
		txCtx, tx := j.tm.Begin(ctx)
		rec.Status = msgEntity.MsgStatusSuccess
		if err := j.repo.UpdateRecord(txCtx, rec); err != nil {
			slog.Error("message job execute: update record failed", "recordID", rec.ID, "err", err)
			j.tm.Rollback(tx)
			return errorx.New(errorx.CodeInternalError, "消息投递失败")
		}
		msgType := 2
		if rec.Receiver == "all" {
			msgType = 1
		}
		internalMsg := &msgEntity.MsgInternal{
			MsgRecordID: rec.ID,
			Type:        msgType,
		}
		if err := j.repo.CreateInternal(txCtx, internalMsg); err != nil {
			slog.Error("message job execute: create internal failed", "recordID", rec.ID, "err", err)
			j.tm.Rollback(tx)
			return errorx.New(errorx.CodeInternalError, "消息投递失败")
		}
		if err := j.tm.Commit(tx); err != nil {
			slog.Error("message job execute: commit failed", "recordID", rec.ID, "err", err)
			return errorx.New(errorx.CodeInternalError, "消息投递失败")
		}
		return nil
	}

	if !j.isChannelEnabled(rec.Channel) {
		rec.Status = msgEntity.MsgStatusFailed
		rec.ErrorMsg = rec.Channel + " service is disabled"
		return j.repo.UpdateRecord(ctx, rec)
	}

	driver, ok := j.drivers[rec.Channel]
	if !ok {
		rec.Status = msgEntity.MsgStatusFailed
		rec.ErrorMsg = "no driver found for channel: " + rec.Channel
		return j.repo.UpdateRecord(ctx, rec)
	}

	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err = driver.Send(sendCtx, rec.Receiver, rec.Title, rec.Content, nil)
	if err != nil {
		rec.Status = msgEntity.MsgStatusFailed
		rec.ErrorMsg = err.Error()
		rec.RetryCount++
	} else {
		rec.Status = msgEntity.MsgStatusSuccess
		rec.ErrorMsg = ""
	}

	return j.repo.UpdateRecord(ctx, rec)
}
