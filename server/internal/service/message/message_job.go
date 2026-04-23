package message

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	msgEntity "NetyAdmin/internal/domain/entity/message"
	"NetyAdmin/internal/pkg/configsync"
	msgPkg "NetyAdmin/internal/pkg/message"
	"NetyAdmin/internal/pkg/task"
	msgRepo "NetyAdmin/internal/repository/message"
)

type MsgSendJob struct {
	repo    msgRepo.MsgRepository
	drivers map[string]msgPkg.Driver
	watcher configsync.ConfigWatcher
}

func NewMsgSendJob(repo msgRepo.MsgRepository, drivers map[string]msgPkg.Driver, watcher configsync.ConfigWatcher) *MsgSendJob {
	return &MsgSendJob{
		repo:    repo,
		drivers: drivers,
		watcher: watcher,
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
		rec.Status = msgEntity.MsgStatusSuccess
		if err := j.repo.UpdateRecord(ctx, rec); err != nil {
			return err
		}
		msgType := 2
		if rec.Receiver == "all" {
			msgType = 1
		}
		internalMsg := &msgEntity.MsgInternal{
			MsgRecordID: rec.ID,
			Type:        msgType,
		}
		return j.repo.CreateInternal(ctx, internalMsg)
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
