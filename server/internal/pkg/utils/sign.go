package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
)

// HMACSHA256Hex 使用 key 对 data 计算 HMAC-SHA256，返回小写 hex 编码字符串。
// 用于上传凭证签名等场景，与 recordID 组合防止只传 ID 的伪造攻击。
func HMACSHA256Hex(key, data string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyHMACSHA256Hex 在恒定时间下校验 HMAC-SHA256(hex) 签名是否匹配。
func VerifyHMACSHA256Hex(key, data, expectHex string) bool {
	actual := HMACSHA256Hex(key, data)
	return hmac.Equal([]byte(actual), []byte(expectHex))
}

// SignUploadRecord 拼装上传凭证签名原文并计算 HMAC。
// 字段顺序固定，包含 recordID/objectKey/source/sourceID/expiresAtUnix，
// 任一字段被篡改都会破坏签名。
func SignUploadRecord(key string, recordID uint, objectKey, source, sourceID string, expiresAtUnix int64) string {
	data := fmt.Sprintf("%d|%s|%s|%s|%s", recordID, objectKey, source, sourceID, strconv.FormatInt(expiresAtUnix, 10))
	return HMACSHA256Hex(key, data)
}

// VerifyUploadRecord 校验上传凭证签名是否匹配（恒定时间比较，防时序攻击）。
func VerifyUploadRecord(key string, recordID uint, objectKey, source, sourceID string, expiresAtUnix int64, secret string) bool {
	expect := SignUploadRecord(key, recordID, objectKey, source, sourceID, expiresAtUnix)
	return hmac.Equal([]byte(expect), []byte(secret))
}
