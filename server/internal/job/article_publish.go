package job

import (
	"context"
	"log"
	"time"

	"NetyAdmin/internal/pkg/task"
	contentRepo "NetyAdmin/internal/repository/content"
)

// ArticlePublishJob 文章定时发布任务
type ArticlePublishJob struct {
	repo contentRepo.ContentArticleRepository
}

func NewArticlePublishJob(repo contentRepo.ContentArticleRepository) *ArticlePublishJob {
	return &ArticlePublishJob{repo: repo}
}

func (j *ArticlePublishJob) Name() string {
	return "article_publish"
}

func (j *ArticlePublishJob) DisplayName() string {
	return "Article Scheduler"
}

func (j *ArticlePublishJob) Run(ctx context.Context) error {
	now := time.Now()

	count, err := j.repo.PublishScheduled(ctx, now)
	if err != nil {
		return err
	}

	if count > 0 {
		log.Printf("[任务:文章发布] 成功发布 %d 篇文章", count)
	}

	return nil
}

// DefaultMetadata 默认间隔执行，权重 80
func (j *ArticlePublishJob) DefaultMetadata() task.TaskMetadata {
	return task.TaskMetadata{
		Name:        j.Name(),
		DisplayName: j.DisplayName(),
		Type:        task.TypeInterval,
		Spec:        "1m",
		Weight:      task.WeightEssential, // 80
		Enabled:     true,
	}
}
