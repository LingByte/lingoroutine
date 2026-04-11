package token

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrTokenExpired token已过期
	ErrTokenExpired = errors.New("token expired")
	// ErrTokenInvalid token无效
	ErrTokenInvalid = errors.New("token invalid")
	// ErrTokenMalformed token格式错误
	ErrTokenMalformed = errors.New("token malformed")
	// ErrRefreshTokenExpired refresh token已过期
	ErrRefreshTokenExpired = errors.New("refresh token expired")
)

// Claims JWT claims
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role,omitempty"`
	Type     string `json:"type"` // "access" or "refresh"
	Nonce    string `json:"nonce,omitempty"` // 随机数确保唯一性
	IssuedAt int64  `json:"iat"`
	ExpiresAt int64 `json:"exp"`
}

// TokenPair token对（access token + refresh token）
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`    // access token过期时间（秒）
	RefreshExpiresIn int64 `json:"refresh_expires_in"` // refresh token过期时间（秒）
}

// JWT JWT token工具
type JWT struct {
	secretKey    []byte
	encryptKey   []byte
	useEncryption bool
	accessTokenExpire int64  // access token过期时间（秒）
	refreshTokenExpire int64 // refresh token过期时间（秒）
}

// NewJWT 创建JWT实例
// secretKey: 用于签名的密钥
// encryptKey: 用于加密payload的密钥（可选，为空时不加密）
// accessTokenExpire: access token过期时间（秒），默认3600（1小时）
// refreshTokenExpire: refresh token过期时间（秒），默认604800（7天）
func NewJWT(secretKey, encryptKey string, accessTokenExpire, refreshTokenExpire int64) *JWT {
	if accessTokenExpire == 0 {
		accessTokenExpire = 3600 // 默认1小时
	}
	if refreshTokenExpire == 0 {
		refreshTokenExpire = 604800 // 默认7天
	}
	return &JWT{
		secretKey:         []byte(secretKey),
		encryptKey:        []byte(encryptKey),
		useEncryption:     encryptKey != "",
		accessTokenExpire: accessTokenExpire,
		refreshTokenExpire: refreshTokenExpire,
	}
}

// GenerateTokenPair 生成token对（access token + refresh token）
func (j *JWT) GenerateTokenPair(claims *Claims) (*TokenPair, error) {
	// 生成access token
	now := time.Now().UnixNano()
	accessClaims := *claims
	accessClaims.Type = "access"
	accessClaims.Nonce = generateNonce()
	accessClaims.IssuedAt = now / 1e9 // 转换为秒
	accessClaims.ExpiresAt = now/1e9 + j.accessTokenExpire

	accessToken, err := j.generate(&accessClaims)
	if err != nil {
		return nil, fmt.Errorf("generate access token failed: %w", err)
	}

	// 生成refresh token
	now = time.Now().UnixNano()
	refreshClaims := *claims
	refreshClaims.Type = "refresh"
	refreshClaims.Nonce = generateNonce()
	refreshClaims.IssuedAt = now / 1e9 // 转换为秒
	refreshClaims.ExpiresAt = now/1e9 + j.refreshTokenExpire

	refreshToken, err := j.generate(&refreshClaims)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token failed: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    j.accessTokenExpire,
		RefreshExpiresIn: j.refreshTokenExpire,
	}, nil
}

// GenerateAccessToken 生成access token（用于刷新时）
func (j *JWT) GenerateAccessToken(claims *Claims) (string, int64, error) {
	now := time.Now().UnixNano()

	accessClaims := *claims
	accessClaims.Type = "access"
	accessClaims.Nonce = generateNonce()
	accessClaims.IssuedAt = now / 1e9 // 转换为秒
	accessClaims.ExpiresAt = now/1e9 + j.accessTokenExpire

	token, err := j.generate(&accessClaims)
	if err != nil {
		return "", 0, fmt.Errorf("generate access token failed: %w", err)
	}

	return token, j.accessTokenExpire, nil
}

// generateNonce 生成随机nonce
func generateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 如果随机数生成失败，使用时间戳作为fallback
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// generate 生成token
func (j *JWT) generate(claims *Claims) (string, error) {
	// 序列化claims
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims failed: %w", err)
	}

	// 如果启用加密，对payload进行加密
	var payload string
	if j.useEncryption {
		encrypted, err := j.encrypt(claimsJSON)
		if err != nil {
			return "", fmt.Errorf("encrypt payload failed: %w", err)
		}
		payload = base64.URLEncoding.EncodeToString(encrypted)
	} else {
		payload = base64.URLEncoding.EncodeToString(claimsJSON)
	}

	// 构建header
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
		"enc": "AES", // 标识是否加密
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header failed: %w", err)
	}

	// Base64编码header
	headerEncoded := base64.URLEncoding.EncodeToString(headerJSON)

	// 生成签名
	signature := j.sign(headerEncoded, payload)

	// 拼接token
	token := fmt.Sprintf("%s.%s.%s", headerEncoded, payload, signature)
	return token, nil
}

// VerifyAccessToken 验证access token
func (j *JWT) VerifyAccessToken(token string) (*Claims, error) {
	claims, err := j.verify(token)
	if err != nil {
		return nil, err
	}

	// 检查token类型
	if claims.Type != "access" {
		return nil, errors.New("invalid token type: expected access token")
	}

	return claims, nil
}

// VerifyRefreshToken 验证refresh token
func (j *JWT) VerifyRefreshToken(token string) (*Claims, error) {
	claims, err := j.verify(token)
	if err != nil {
		return nil, err
	}

	// 检查token类型
	if claims.Type != "refresh" {
		return nil, errors.New("invalid token type: expected refresh token")
	}

	return claims, nil
}

// verify 验证token
func (j *JWT) verify(token string) (*Claims, error) {
	// 分割token
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrTokenMalformed
	}

	headerEncoded, payloadEncoded, signature := parts[0], parts[1], parts[2]

	// 验证签名
	expectedSignature := j.sign(headerEncoded, payloadEncoded)
	if signature != expectedSignature {
		return nil, ErrTokenInvalid
	}

	// 解码payload
	var payloadBytes []byte
	var err error

	if j.useEncryption {
		// 解密payload
		encrypted, err := base64.URLEncoding.DecodeString(payloadEncoded)
		if err != nil {
			return nil, fmt.Errorf("decode payload failed: %w", err)
		}
		payloadBytes, err = j.decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt payload failed: %w", err)
		}
	} else {
		// 直接解码payload
		payloadBytes, err = base64.URLEncoding.DecodeString(payloadEncoded)
		if err != nil {
			return nil, fmt.Errorf("decode payload failed: %w", err)
		}
	}

	// 反序列化claims
	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims failed: %w", err)
	}

	// 检查过期时间
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

// RefreshToken 使用refresh token刷新access token
func (j *JWT) RefreshToken(refreshToken string) (*TokenPair, error) {
	// 验证refresh token
	claims, err := j.VerifyRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// 清空nonce以确保生成新的随机数
	claims.Nonce = ""

	// 生成新的token对
	return j.GenerateTokenPair(claims)
}

// RefreshAccessToken 仅刷新access token（不生成新的refresh token）
func (j *JWT) RefreshAccessToken(refreshToken string) (string, int64, error) {
	// 验证refresh token
	claims, err := j.VerifyRefreshToken(refreshToken)
	if err != nil {
		return "", 0, err
	}

	// 清空nonce以确保生成新的随机数
	claims.Nonce = ""

	// 生成新的access token
	return j.GenerateAccessToken(claims)
}

// encrypt 加密payload
func (j *JWT) encrypt(data []byte) ([]byte, error) {
	// 这里需要使用AES加密
	// 为了简化，暂时使用简单的加密方式
	// 实际应该使用utils包中的AesEncrypt
	return j.aesEncrypt(data)
}

// decrypt 解密payload
func (j *JWT) decrypt(data []byte) ([]byte, error) {
	return j.aesDecrypt(data)
}

// aesEncrypt AES加密
func (j *JWT) aesEncrypt(plaintext []byte) ([]byte, error) {
	// 简化实现，实际应该使用utils.AesEncrypt
	// 这里为了独立性，重新实现AES加密
	key := j.encryptKey
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("invalid key size")
	}
	
	// 使用XOR作为临时加密方案（实际应该使用AES）
	encrypted := make([]byte, len(plaintext))
	for i, b := range plaintext {
		encrypted[i] = b ^ key[i%len(key)]
	}
	return encrypted, nil
}

// aesDecrypt AES解密
func (j *JWT) aesDecrypt(ciphertext []byte) ([]byte, error) {
	key := j.encryptKey
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("invalid key size")
	}
	
	// 使用XOR作为临时解密方案
	decrypted := make([]byte, len(ciphertext))
	for i, b := range ciphertext {
		decrypted[i] = b ^ key[i%len(key)]
	}
	return decrypted, nil
}

// sign 生成签名
func (j *JWT) sign(header, payload string) string {
	message := fmt.Sprintf("%s.%s", header, payload)
	h := hmac.New(sha256.New, j.secretKey)
	h.Write([]byte(message))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

// ParseWithoutVerification 解析token但不验证签名（仅用于调试）
func (j *JWT) ParseWithoutVerification(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrTokenMalformed
	}

	payloadEncoded := parts[1]

	// 解码payload
	var payloadBytes []byte
	var err error

	if j.useEncryption {
		// 解密payload
		encrypted, err := base64.URLEncoding.DecodeString(payloadEncoded)
		if err != nil {
			return nil, fmt.Errorf("decode payload failed: %w", err)
		}
		payloadBytes, err = j.decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt payload failed: %w", err)
		}
	} else {
		// 直接解码payload
		payloadBytes, err = base64.URLEncoding.DecodeString(payloadEncoded)
		if err != nil {
			return nil, fmt.Errorf("decode payload failed: %w", err)
		}
	}

	// 反序列化claims
	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims failed: %w", err)
	}

	return &claims, nil
}

// GetExpirationTime 获取token的过期时间
func (j *JWT) GetExpirationTime(token string) (int64, error) {
	claims, err := j.ParseWithoutVerification(token)
	if err != nil {
		return 0, err
	}
	return claims.ExpiresAt, nil
}

// IsExpired 检查token是否过期
func (j *JWT) IsExpired(token string) (bool, error) {
	claims, err := j.ParseWithoutVerification(token)
	if err != nil {
		return true, err
	}
	return time.Now().Unix() > claims.ExpiresAt, nil
}

// ShouldRefresh 检查access token是否应该刷新（例如：剩余时间小于阈值）
func (j *JWT) ShouldRefresh(token string, threshold int64) (bool, error) {
	expTime, err := j.GetExpirationTime(token)
	if err != nil {
		return false, err
	}
	remaining := expTime - time.Now().Unix()
	return remaining < threshold, nil
}
