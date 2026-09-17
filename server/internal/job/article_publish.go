package job

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"NetyAdmin/internal/pkg/cache"
	"NetyAdmin/internal/pkg/task"
	contentRepo "NetyAdmin/internal/repository/content"
)

// ArticlePublishJob 文章定时发布任务
type ArticlePublishJob struct {
	repo      contentRepo.ContentArticleRepository
	cacheFast cache.ConfigCache
}

func NewArticlePublishJob(repo contentRepo.ContentArticleRepository, cacheFast cache.ConfigCache) *ArticlePublishJob {
	return &ArticlePublishJob{repo: repo, cacheFast: cacheFast}
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
		// 发布结果必须失效文章缓存：C 端列表/详情走 FetchFast 缓存，
		// 不失效则定时发布的文章最长延迟一个缓存 TTL 才可见
		// （与 admin 手动 Publish 路径的失效行为保持一致）。
		if err := j.cacheFast.InvalidateByTags(ctx, cache.TagContentArticle); err != nil {
			slog.Error("article publish job: invalidate cache failed", "tag", cache.TagContentArticle, "err", err)
		}
		slog.Info("文章发布任务完成", "count", count)
	}

	return nil
}

func (j *ArticlePublishJob) Execute(ctx context.Context, payload json.RawMessage) error {
	return nil
}

// DefaultMetadata 默认间隔执行，权重 80
func (j *ArticlePublishJob) DefaultMetadata() task.TaskMetadata {
	return task.TaskMetadata{
		Name:        j.Name(),
		DisplayName: j.DisplayName(),
		Type:        task.TypeInterval,
		Spec:        "1m",
		Weight:      task.WeightNormal, // 50 (业务级任务)
		Enabled:     true,
	}
}
