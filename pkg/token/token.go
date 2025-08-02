package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// secretKey 是一个全局变量，用于存储服务器在启动时生成的32字节密钥。
var secretKey []byte

// TokenPayload 定义了需要被签名的数据结构。
// 它将在 /pair 请求的响应中和 /vote 请求的请求体中被序列化和反序列化。
type TokenPayload struct {
	PairID   string `json:"p"`
	SpellAID string `json:"a"`
	SpellBID string `json:"b"`
}

// GenerateSecretKey 生成一个密码学安全的32字节随机密钥。
func GenerateSecretKey() {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		panic("无法生成安全的密钥: " + err.Error())
	}
	secretKey = key
	fmt.Println("HMAC密钥已成功生成。")
}

// GenerateVoteSignature 为一个给定的TokenPayload生成一个HMAC签名。
// 它返回的是签名的Base64编码字符串。
func GenerateVoteSignature(payload TokenPayload) (string, error) {
	// 1. 将payload序列化为JSON字符串
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", errors.New("无法序列化Token payload")
	}

	// 2. 使用HMAC-SHA256和密钥对payload进行签名
	mac := hmac.New(sha256.New, secretKey)
	mac.Write(payloadBytes)
	signature := mac.Sum(nil)

	// 3. 对签名进行Base64编码，并返回
	encodedSignature := base64.RawURLEncoding.EncodeToString(signature)
	return encodedSignature, nil
}

// ValidateVoteSignature 验证一个给定的payload和签名是否匹配。
func ValidateVoteSignature(payload TokenPayload, signatureB64 string) bool {
	// 1. 将传入的payload重新序列化，以确保与签名时的数据格式完全一致
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false // 如果序列化失败，则验证失败
	}

	// 2. 重新计算预期的签名
	mac := hmac.New(sha256.New, secretKey)
	mac.Write(payloadBytes)
	expectedSignature := mac.Sum(nil)

	// 3. 解码前端传来的签名
	actualSignature, err := base64.RawURLEncoding.DecodeString(signatureB64)
	if err != nil {
		return false // 签名解码失败
	}

	// 4. 使用 hmac.Equal 进行安全的、时间恒定的比较，防止时序攻击
	return hmac.Equal(expectedSignature, actualSignature)
}
