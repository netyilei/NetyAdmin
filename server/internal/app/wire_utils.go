package app

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"NetyAdmin/internal/config"
	"NetyAdmin/internal/pkg/pubsub"
	"NetyAdmin/internal/pkg/utils"
)

// generateNodeID 生成进程级唯一节点标识，用于事件总线过滤本节点回环消息
// 格式: hostname-ULID后8位（hostname 提供主机维度，ULID 后缀提供进程维度）
func generateNodeID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	ulid := utils.NewULID()
	if len(ulid) > 8 {
		ulid = ulid[len(ulid)-8:]
	}
	return fmt.Sprintf("%s-%s", host, ulid)
}

// safeSubscribe 包装 eventBus.Subscribe：返回 error 实现 fail-closed。
// panic 恢复下沉到 pubsub/bus.go 的 dispatch 层（通过 recovery.GoSafe）。
//
// 设计说明：PubSub 消息分发是异步的，dispatch 在 goroutine 中调用 handler。
// 早期版本在 safeSubscribe 内部做 sync recover 包裹，但由于 dispatch 启动
// goroutine 时已用 recovery.GoSafe 包裹（SubTask 5.3），此处再包一层
// recover 属于冗余。简化为透传 handler，让恢复逻辑统一由 dispatch 的 GoSafe
// 负责——所有 PubSub 异步路径的 panic 都走同一条「slog.Error + Sentry
// CaptureException」管线，避免恢复逻辑分散在多处导致行为不一致。
//
// handler 接收 ctx，dispatch 已在其中通过 msg.Meta 恢复 request_id 到子 ctx
// （Task 8.4），handler 内部可用 slogutil.LoggerFromContext(ctx) 关联到原始请求。
//
// fail-closed 语义（P1-7）：订阅失败（如 topic 已注册 / 注册表写入异常）会
// 导致关键事件（ConfigSync / StorageSync / CacheInvalidation / IPACReload /
// CacheDelete）不被消费，服务"看起来正常"实则数据不一致。因此 Subscribe
// 失败必须阻断 Bootstrap 启动，调用方须 `return nil, err`。
//
// 保留此函数是为了维持 wire.go 调用点的可读性（语义化命名），并作为未来
// 若需要在 Subscribe 注册阶段做扩展（如 metrics 埋点）的扩展点。
func safeSubscribe(bus pubsub.EventBus, topic string, handler func(ctx context.Context, msg []byte)) error {
	if err := bus.Subscribe(topic, handler); err != nil {
		return fmt.Errorf("subscribe %s: %w", topic, err)
	}
	return nil
}

// loadRSAPrivateKey 从 config 加载 RSA 私钥用于 RS256 签发。
// 优先读 PrivateKeyFile；为空则用 PrivateKeyPEM 内联。两者均空返回 error（fail-closed）。
// 同时兼容 PKCS#1（RSA PRIVATE KEY）与 PKCS#8（PRIVATE KEY）两种 PEM 编码。
func loadRSAPrivateKey(cfg *config.JWTConfig) (*rsa.PrivateKey, error) {
	pemBytes, err := loadPEMSource(cfg.PrivateKeyFile, cfg.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("读取私钥失败: %w", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("私钥 PEM 解码失败：不是合法的 PEM 格式")
	}

	// 优先尝试 PKCS#1（传统 RSA PRIVATE KEY 格式）
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	// 回退到 PKCS#8（通用 PRIVATE KEY 格式，可能含 ECDSA/RSA 等）
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("私钥解析失败（PKCS#1/PKCS#8 均不支持）: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("私钥不是 RSA 类型（实际类型 %T），RS256 签名仅支持 RSA 私钥", key)
	}
	return rsaKey, nil
}

// loadRSAPublicKey 从 config 加载 RSA 公钥用于 RS256 验签。
// 优先读 PublicKeyFile；为空则用 PublicKeyPEM 内联。两者均空返回 error（fail-closed）。
// 公钥统一使用 PKIX 编码（PUBLIC KEY PEM block）。
func loadRSAPublicKey(cfg *config.JWTConfig) (*rsa.PublicKey, error) {
	pemBytes, err := loadPEMSource(cfg.PublicKeyFile, cfg.PublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("读取公钥失败: %w", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("公钥 PEM 解码失败：不是合法的 PEM 格式")
	}

	// PKIX 公钥解析（适用于 "PUBLIC KEY" PEM block，X.509 SubjectPublicKeyInfo）
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// 兼容 PKCS#1 RSA PUBLIC KEY（少数工具导出格式）
		if rsaPub, perr := x509.ParsePKCS1PublicKey(block.Bytes); perr == nil {
			return rsaPub, nil
		}
		return nil, fmt.Errorf("公钥解析失败（PKIX/PKCS#1 均不支持）: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("公钥不是 RSA 类型（实际类型 %T），RS256 验签仅支持 RSA 公钥", key)
	}
	return rsaKey, nil
}

// loadPEMSource 统一加载 PEM 内容：优先读 file path，为空则用内联 PEM 字符串。
// file path 与 inline 同时为空返回 error（fail-closed）。
func loadPEMSource(filePath, inlinePEM string) ([]byte, error) {
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("读取文件 %q 失败: %w", filePath, err)
		}
		return data, nil
	}
	if inlinePEM == "" {
		return nil, fmt.Errorf("file path 与 inline PEM 均为空（fail-closed）")
	}
	return []byte(inlinePEM), nil
}
