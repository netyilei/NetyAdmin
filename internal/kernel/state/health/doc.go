// Package health 实现状态模式解析与就绪聚合：
//
//	模式解析：deployment.topology × state.mode 的运行时健康解读
//	（启动期 fail-fast 校验在 kernel/config.Validate；本包做运行期聚合）；
//	就绪聚合：/health/ready 的依赖检查（PG/Redis/…）注册与汇聚。
package health
