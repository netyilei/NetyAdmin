// Package authn 实现认证（AuthN）：Provider 化（Admin JWT / User JWT / Service JWT /
// App HMAC / API Key），统一 Authenticator 与 Principal 上下文，TokenStore 吊销。
// JWT RS256 本地校验；吊销经 identity.token.revoked 事件。
// 实现：（依赖 security/primitives）。
package authn
