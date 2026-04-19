package message

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	msgEntity "NetyAdmin/internal/domain/entity/message"
	msgPkg "NetyAdmin/internal/pkg/message"
	"NetyAdmin/internal/pkg/task"
	msgRepo "NetyAdmin/internal/repository/message"
)

// MsgSendJob 消息发送任务处理器
type MsgSendJob struct {
	repo    msgRepo.MsgRepository
	drivers map[string]msgPkg.Driver
}

func NewMsgSendJob(repo msgRepo.MsgRepository, drivers map[string]msgPkg.Driver) *MsgSendJob {
	return &MsgSendJob{
		repo:    repo,
		drivers: drivers,
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
		Type:        task.TypeOnce, // 仅作为消费者，不需要定时触发生产者逻辑
		Enabled:     true,
		Weight:      task.WeightEssential,
	}
}

func (j *MsgSendJob) Run(ctx context.Context) error {
	// 生产者逻辑：如果需要定期扫描库里漏掉的“等待”记录，可以在这里实现
	// 目前通过 Dispatch 实时触发，所以这里可以留空或做扫描补偿
	return nil
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
