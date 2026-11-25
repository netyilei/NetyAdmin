// Package keys 装载 JWT RS256 签名密钥并管理轮换重叠窗口（
// docs/rules/security.md §1 + hot-update §密钥重叠）。
//
// 文件约定（keysDir 下）：
//
//	jwt_active.pem          签发+验签私钥（必须；缺 → fail-closed 拒绝启动）
//	jwt_retired_public.pem  旧钥公钥（可缺；存在则并入验签集——重叠窗口）
//
// 轮换流程（N-1 重叠，代码不热更=密钥不热更，重启生效）：
// ①新钥写 jwt_active_new.pem → ②改名：旧 active→retired、new→active →
// ③旧私钥立即移出（retired 只留公钥——私钥暴露面最小化）→ ④重启：Signer=新钥、
// Verifiers=新+旧双公钥 → ⑤旧 token 全过期后移除 retired。
// 验签侧重叠语义由 JWTVerifier 多公钥（任一通过）承接。
//
// 消费者：ID-01 管理端签发装配（当前无运行时消费者——机制行交付）；
// TLS/mTLS 传输面随/。
package keys

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// 文件名约定（keysDir 下；密钥目录已 gitignore——AGENTS §14 本地环境）。
const (
	FileNameActive        = "jwt_active.pem"
	FileNameRetiredPublic = "jwt_retired_public.pem"
)

// SigningKeys 是装载完成的密钥集（构造后不可变）。
type SigningKeys struct {
	signer    *rsa.PrivateKey
	verifiers []*rsa.PublicKey // active 公钥在首位
}

// Load 从 dir 装载密钥集（fail-closed：active 缺失/格式错/非 RSA 私钥 → 错误；
// retired 破损 → 错误——宁可拒绝启动不给静默降级验签面）。
func Load(dir string) (*SigningKeys, error) {
	if dir == "" {
		return nil, errors.New("keys: empty dir")
	}
	signer, err := loadPrivate(filepath.Join(dir, FileNameActive))
	if err != nil {
		return nil, fmt.Errorf("keys: active: %w", err)
	}
	verifiers := []*rsa.PublicKey{&signer.PublicKey}

	retired, err := loadPublic(filepath.Join(dir, FileNameRetiredPublic))
	if err == nil {
		verifiers = append(verifiers, retired)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("keys: retired: %w", err)
	}
	return &SigningKeys{signer: signer, verifiers: verifiers}, nil
}

// Signer 返回签发私钥（active）。
func (k *SigningKeys) Signer() *rsa.PrivateKey { return k.signer }

// Verifiers 返回验签公钥集（active + retired 重叠窗口； 多公钥任一通过）。
func (k *SigningKeys) Verifiers() []*rsa.PublicKey {
	return append([]*rsa.PublicKey(nil), k.verifiers...)
}

// loadPrivate 解析 PEM 私钥（PKCS#1 与 PKCS#8 双格式宽容；公钥文件按错误拒绝）。
func loadPrivate(path string) (*rsa.PrivateKey, error) {
	block, err := pemBlock(path)
	if err != nil {
		return nil, err
	}
	switch block.Type {
	case "RSA PRIVATE KEY": // PKCS#1（openssl genrsa 传统输出）
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%s: pkcs1: %w", filepath.Base(path), err)
		}
		return key, nil
	case "PRIVATE KEY": // PKCS#8
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%s: pkcs8: %w", filepath.Base(path), err)
		}
		key, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("%s: not an rsa private key", filepath.Base(path))
		}
		return key, nil
	default:
		return nil, fmt.Errorf("%s: unexpected pem type %q", filepath.Base(path), block.Type)
	}
}

// loadPublic 解析 PEM RSA 公钥（PKCS#1 "PUBLIC KEY" / SPKI "RSA PUBLIC KEY"）。
func loadPublic(path string) (*rsa.PublicKey, error) {
	block, err := pemBlock(path)
	if err != nil {
		return nil, err
	}
	switch block.Type {
	case "PUBLIC KEY": // SPKI
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%s: spki: %w", filepath.Base(path), err)
		}
		key, ok := parsed.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("%s: not an rsa public key", filepath.Base(path))
		}
		return key, nil
	case "RSA PUBLIC KEY": // PKCS#1
		return x509.ParsePKCS1PublicKey(block.Bytes)
	default:
		return nil, fmt.Errorf("%s: unexpected pem type %q", filepath.Base(path), block.Type)
	}
}

func pemBlock(path string) (*pem.Block, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("%s: no pem block", filepath.Base(path))
	}
	return block, nil
}
