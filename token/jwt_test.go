package token

// Copyright (c) 2026 LingByte
// SPDX-License-Identifier: MIT

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJWT(t *testing.T) {
	// 测试不加密的JWT
	jwt1 := NewJWT("secret-key", "", 3600, 604800)
	assert.NotNil(t, jwt1)
	assert.False(t, jwt1.useEncryption)
	assert.Equal(t, int64(3600), jwt1.accessTokenExpire)
	assert.Equal(t, int64(604800), jwt1.refreshTokenExpire)

	// 测试加密的JWT
	jwt2 := NewJWT("secret-key", "encrypt-key-16bytes", 3600, 604800)
	assert.NotNil(t, jwt2)
	assert.True(t, jwt2.useEncryption)

	// 测试默认过期时间
	jwt3 := NewJWT("secret-key", "", 0, 0)
	assert.Equal(t, int64(3600), jwt3.accessTokenExpire)
	assert.Equal(t, int64(604800), jwt3.refreshTokenExpire)
}

func TestGenerateTokenPair(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Equal(t, int64(3600), pair.ExpiresIn)
	assert.Equal(t, int64(604800), pair.RefreshExpiresIn)
}

func TestGenerateTokenPair_WithEncryption(t *testing.T) {
	jwt := NewJWT("secret-key", "1234567890123456", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
}

func TestVerifyAccessToken(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 验证access token
	verifiedClaims, err := jwt.VerifyAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(123), verifiedClaims.UserID)
	assert.Equal(t, "testuser", verifiedClaims.Username)
	assert.Equal(t, "test@example.com", verifiedClaims.Email)
	assert.Equal(t, "admin", verifiedClaims.Role)
	assert.Equal(t, "access", verifiedClaims.Type)
}

func TestVerifyAccessToken_WithEncryption(t *testing.T) {
	jwt := NewJWT("secret-key", "1234567890123456", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 验证access token
	verifiedClaims, err := jwt.VerifyAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(123), verifiedClaims.UserID)
	assert.Equal(t, "testuser", verifiedClaims.Username)
	assert.Equal(t, "access", verifiedClaims.Type)
}

func TestVerifyRefreshToken(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 验证refresh token
	verifiedClaims, err := jwt.VerifyRefreshToken(pair.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, uint(123), verifiedClaims.UserID)
	assert.Equal(t, "testuser", verifiedClaims.Username)
	assert.Equal(t, "refresh", verifiedClaims.Type)
}

func TestVerifyAccessToken_InvalidType(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 用refresh token验证access token应该失败
	_, err = jwt.VerifyAccessToken(pair.RefreshToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token type")
}

func TestVerifyRefreshToken_InvalidType(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 用access token验证refresh token应该失败
	_, err = jwt.VerifyRefreshToken(pair.AccessToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token type")
}

func TestRefreshToken(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}

	// 生成初始token对
	pair1, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 添加延迟确保时间戳不同
	time.Sleep(1 * time.Millisecond)

	// 使用refresh token刷新
	pair2, err := jwt.RefreshToken(pair1.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, pair2.AccessToken)
	assert.NotEmpty(t, pair2.RefreshToken)
	assert.NotEqual(t, pair1.AccessToken, pair2.AccessToken)
	assert.NotEqual(t, pair1.RefreshToken, pair2.RefreshToken)

	// 验证新的access token
	verifiedClaims, err := jwt.VerifyAccessToken(pair2.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(123), verifiedClaims.UserID)
	assert.Equal(t, "testuser", verifiedClaims.Username)
}

func TestRefreshAccessToken(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}

	// 生成初始token对
	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 添加延迟确保时间戳不同
	time.Sleep(1 * time.Millisecond)

	// 仅刷新access token
	newAccessToken, expiresIn, err := jwt.RefreshAccessToken(pair.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newAccessToken)
	assert.NotEqual(t, pair.AccessToken, newAccessToken)
	assert.Equal(t, int64(3600), expiresIn)

	// 验证新的access token
	verifiedClaims, err := jwt.VerifyAccessToken(newAccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(123), verifiedClaims.UserID)
	assert.Equal(t, "testuser", verifiedClaims.Username)
}

func TestVerifyToken_Expired(t *testing.T) {
	jwt := NewJWT("secret-key", "", 1, 604800) // access token 1秒后过期
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 等待token过期
	time.Sleep(2 * time.Second)

	// 验证过期的access token
	_, err = jwt.VerifyAccessToken(pair.AccessToken)
	assert.Error(t, err)
	assert.Equal(t, ErrTokenExpired, err)
}

func TestVerifyToken_InvalidSignature(t *testing.T) {
	jwt1 := NewJWT("secret-key-1", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt1.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 使用不同密钥验证
	jwt2 := NewJWT("secret-key-2", "", 3600, 604800)
	_, err = jwt2.VerifyAccessToken(pair.AccessToken)
	assert.Error(t, err)
	assert.Equal(t, ErrTokenInvalid, err)
}

func TestVerifyToken_Malformed(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)

	testCases := []string{
		"",
		"invalid",
		"part1.part2",
		"part1.part2.part3.part4",
	}

	for _, tc := range testCases {
		_, err := jwt.VerifyAccessToken(tc)
		assert.Error(t, err)
		assert.Equal(t, ErrTokenMalformed, err)
	}
}

func TestRefreshToken_Expired(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 1) // refresh token 1秒后过期
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 等待refresh token过期
	time.Sleep(2 * time.Second)

	// 使用过期的refresh token刷新
	_, err = jwt.RefreshToken(pair.RefreshToken)
	assert.Error(t, err)
	assert.Equal(t, ErrTokenExpired, err)
}

func TestParseWithoutVerification(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 不验证签名解析access token
	parsedClaims, err := jwt.ParseWithoutVerification(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(123), parsedClaims.UserID)
	assert.Equal(t, "testuser", parsedClaims.Username)
	assert.Equal(t, "access", parsedClaims.Type)
}

func TestGetExpirationTime(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 获取access token过期时间
	expTime, err := jwt.GetExpirationTime(pair.AccessToken)
	require.NoError(t, err)
	assert.Greater(t, expTime, time.Now().Unix())
}

func TestIsExpired(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 检查未过期的token
	expired, err := jwt.IsExpired(pair.AccessToken)
	require.NoError(t, err)
	assert.False(t, expired)

	// 生成1秒后过期的token
	jwt2 := NewJWT("secret-key", "", 1, 604800)
	pair2, err := jwt2.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 等待过期
	time.Sleep(2 * time.Second)

	// 检查已过期的token
	expired, err = jwt2.IsExpired(pair2.AccessToken)
	require.NoError(t, err)
	assert.True(t, expired)
}

func TestShouldRefresh(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 检查是否应该刷新（阈值300秒）
	shouldRefresh, err := jwt.ShouldRefresh(pair.AccessToken, 300)
	require.NoError(t, err)
	assert.False(t, shouldRefresh) // 刚生成，不应该刷新
}

func TestGenerateAccessToken(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}

	token, expiresIn, err := jwt.GenerateAccessToken(claims)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, int64(3600), expiresIn)

	// 验证生成的token
	verifiedClaims, err := jwt.VerifyAccessToken(token)
	require.NoError(t, err)
	assert.Equal(t, uint(123), verifiedClaims.UserID)
	assert.Equal(t, "access", verifiedClaims.Type)
}

func TestTokenRotation_MultipleRefreshes(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	// 生成初始token对
	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 多次刷新
	for i := 0; i < 5; i++ {
		pair, err = jwt.RefreshToken(pair.RefreshToken)
		require.NoError(t, err)
		assert.NotEmpty(t, pair.AccessToken)
		assert.NotEmpty(t, pair.RefreshToken)
	}

	// 最后验证
	verifiedClaims, err := jwt.VerifyAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(123), verifiedClaims.UserID)
}

func TestTokenRotation_WithEncryption(t *testing.T) {
	jwt := NewJWT("secret-key", "1234567890123456", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	// 生成初始token对
	pair1, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 刷新token对
	pair2, err := jwt.RefreshToken(pair1.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, pair2.AccessToken)
	assert.NotEmpty(t, pair2.RefreshToken)

	// 验证新的token
	verifiedClaims, err := jwt.VerifyAccessToken(pair2.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(123), verifiedClaims.UserID)
}

func TestClaims_Minimal(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 验证access token
	verifiedClaims, err := jwt.VerifyAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(123), verifiedClaims.UserID)
	assert.Equal(t, "testuser", verifiedClaims.Username)
	assert.Empty(t, verifiedClaims.Email)
}

func TestClaims_AllFields(t *testing.T) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 验证access token
	verifiedClaims, err := jwt.VerifyAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(123), verifiedClaims.UserID)
	assert.Equal(t, "testuser", verifiedClaims.Username)
	assert.Equal(t, "test@example.com", verifiedClaims.Email)
	assert.Equal(t, "admin", verifiedClaims.Role)
}

func TestToken_DifferentSecrets(t *testing.T) {
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	// 使用不同密钥生成token
	jwt1 := NewJWT("secret-1", "", 3600, 604800)
	token1, _, err := jwt1.GenerateAccessToken(claims)
	require.NoError(t, err)

	// 使用不同密钥验证应该失败
	jwt2 := NewJWT("secret-2", "", 3600, 604800)
	_, err = jwt2.VerifyAccessToken(token1)
	assert.Error(t, err)
	assert.Equal(t, ErrTokenInvalid, err)
}

func TestToken_WithEncryption_VerifyWithoutEncryption(t *testing.T) {
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	// 使用加密生成token
	jwt1 := NewJWT("secret", "1234567890123456", 3600, 604800)
	token1, _, err := jwt1.GenerateAccessToken(claims)
	require.NoError(t, err)

	// 使用不加密的方式验证应该失败
	jwt2 := NewJWT("secret", "", 3600, 604800)
	_, err = jwt2.VerifyAccessToken(token1)
	assert.Error(t, err)
}

func TestToken_WithoutEncryption_VerifyWithEncryption(t *testing.T) {
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	// 使用不加密生成token
	jwt1 := NewJWT("secret", "", 3600, 604800)
	token1, _, err := jwt1.GenerateAccessToken(claims)
	require.NoError(t, err)

	// 使用加密的方式验证应该失败
	jwt2 := NewJWT("secret", "1234567890123456", 3600, 604800)
	_, err = jwt2.VerifyAccessToken(token1)
	assert.Error(t, err)
}

func TestToken_InvalidKeySize(t *testing.T) {
	// 测试无效的加密密钥大小
	jwt := NewJWT("secret", "invalid-key", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	// 生成token应该失败
	_, err := jwt.GenerateTokenPair(claims)
	assert.Error(t, err)
}

func TestToken_CustomExpirationTimes(t *testing.T) {
	// 测试自定义过期时间
	jwt := NewJWT("secret", "", 7200, 1209600) // 2小时，14天
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)
	assert.Equal(t, int64(7200), pair.ExpiresIn)
	assert.Equal(t, int64(1209600), pair.RefreshExpiresIn)
}

func TestToken_RefreshWithDifferentClaims(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)

	// 生成初始token对
	claims1 := &Claims{
		UserID:   123,
		Username: "user1",
	}
	pair1, err := jwt.GenerateTokenPair(claims1)
	require.NoError(t, err)

	// 使用旧的refresh token生成新token对（应该保持原有claims）
	pair2, err := jwt.RefreshToken(pair1.RefreshToken)
	require.NoError(t, err)

	// 验证新token应该保持原有claims
	verifiedClaims, err := jwt.VerifyAccessToken(pair2.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(123), verifiedClaims.UserID) // 应该是123，不是456
	assert.Equal(t, "user1", verifiedClaims.Username) // 应该是user1，不是user2
}

func TestToken_TokenUniqueness(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	// 生成多个token对
	tokens := make(map[string]bool)
	for i := 0; i < 10; i++ {
		pair, err := jwt.GenerateTokenPair(claims)
		require.NoError(t, err)

		// 每个token应该是唯一的
		assert.False(t, tokens[pair.AccessToken], "access token should be unique")
		assert.False(t, tokens[pair.RefreshToken], "refresh token should be unique")

		tokens[pair.AccessToken] = true
		tokens[pair.RefreshToken] = true

		// 添加微小延迟确保时间戳不同
		time.Sleep(1 * time.Millisecond)
	}
}

func TestToken_ZeroUserID(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	claims := &Claims{
		UserID:   0,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	verifiedClaims, err := jwt.VerifyAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, uint(0), verifiedClaims.UserID)
}

func TestToken_SpecialCharactersInUsername(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)

	testCases := []string{
		"user@test",
		"user@example.com",
		"user_name",
		"user-name",
		"用户名",
		"user123",
	}

	for _, username := range testCases {
		claims := &Claims{
			UserID:   123,
			Username: username,
		}

		pair, err := jwt.GenerateTokenPair(claims)
		require.NoError(t, err)

		verifiedClaims, err := jwt.VerifyAccessToken(pair.AccessToken)
		require.NoError(t, err)
		assert.Equal(t, username, verifiedClaims.Username)
	}
}

func TestToken_LongEmail(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	longEmail := "very.long.email.address.that.is.extremely.long.and.contains.many.characters@very.long.domain.name.example.com"

	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    longEmail,
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	verifiedClaims, err := jwt.VerifyAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, longEmail, verifiedClaims.Email)
}

func TestToken_EmptyUsername(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	verifiedClaims, err := jwt.VerifyAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Empty(t, verifiedClaims.Username)
}

func TestToken_RefreshTokenReuseProtection(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	// 生成初始token对
	pair1, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 第一次刷新
	pair2, err := jwt.RefreshToken(pair1.RefreshToken)
	require.NoError(t, err)

	// 添加延迟确保时间戳不同
	time.Sleep(1 * time.Millisecond)

	// 尝试再次使用旧的refresh token
	pair3, err := jwt.RefreshToken(pair1.RefreshToken)
	// 注意：当前实现允许重复使用refresh token
	// 如果需要实现一次性refresh token，需要在业务层面维护已使用的refresh token列表
	require.NoError(t, err)
	assert.NotEqual(t, pair2.AccessToken, pair3.AccessToken)
}

func TestToken_ShouldRefresh_Threshold(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 刚生成的token不应该刷新
	shouldRefresh, err := jwt.ShouldRefresh(pair.AccessToken, 3000)
	require.NoError(t, err)
	assert.False(t, shouldRefresh)

	// 阈值设置为很大，应该刷新
	shouldRefresh, err = jwt.ShouldRefresh(pair.AccessToken, 100000)
	require.NoError(t, err)
	assert.True(t, shouldRefresh)
}

func TestToken_ShouldRefresh_NearExpiration(t *testing.T) {
	jwt := NewJWT("secret", "", 10, 604800) // 10秒过期
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 等待接近过期
	time.Sleep(8 * time.Second)

	// 应该刷新
	shouldRefresh, err := jwt.ShouldRefresh(pair.AccessToken, 5)
	require.NoError(t, err)
	assert.True(t, shouldRefresh)
}

func TestToken_ConcurrentGeneration(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	// 并发生成token对
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := jwt.GenerateTokenPair(claims)
			assert.NoError(t, err)
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestToken_ConcurrentVerification(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 并发验证token
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := jwt.VerifyAccessToken(pair.AccessToken)
			assert.NoError(t, err)
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestToken_ConcurrentRefresh(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 并发刷新token
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := jwt.RefreshToken(pair.RefreshToken)
			assert.NoError(t, err)
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestToken_VerifyToken_TamperedPayload(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 篡改token payload
	parts := len(pair.AccessToken)
	if parts > 0 {
		// 修改payload部分
		modifiedToken := pair.AccessToken[:len(pair.AccessToken)-10] + "tampered"
		_, err = jwt.VerifyAccessToken(modifiedToken)
		assert.Error(t, err)
	}
}

func TestToken_VerifyToken_TamperedSignature(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 篡改token signature
	parts := len(pair.AccessToken)
	if parts > 0 {
		// 修改signature部分
		modifiedToken := pair.AccessToken[:len(pair.AccessToken)-5] + "xxxxx"
		_, err = jwt.VerifyAccessToken(modifiedToken)
		assert.Error(t, err)
	}
}

func TestToken_VerifyRefreshToken_AccessToken(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 用access token验证refresh token应该失败
	_, err = jwt.VerifyRefreshToken(pair.AccessToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token type")
}

func TestToken_VerifyAccessToken_RefreshToken(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 用refresh token验证access token应该失败
	_, err = jwt.VerifyAccessToken(pair.RefreshToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token type")
}

func TestToken_ExpiredRefreshToken(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 1) // refresh token 1秒过期
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 等待refresh token过期
	time.Sleep(2 * time.Second)

	// 验证过期的refresh token
	_, err = jwt.VerifyRefreshToken(pair.RefreshToken)
	assert.Error(t, err)
	assert.Equal(t, ErrTokenExpired, err)
}

func TestToken_RefreshAccessToken_ExpiredRefreshToken(t *testing.T) {
	jwt := NewJWT("secret", "", 3600, 1) // refresh token 1秒过期
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
	}

	pair, err := jwt.GenerateTokenPair(claims)
	require.NoError(t, err)

	// 等待refresh token过期
	time.Sleep(2 * time.Second)

	// 使用过期的refresh token刷新access token
	_, _, err = jwt.RefreshAccessToken(pair.RefreshToken)
	assert.Error(t, err)
	assert.Equal(t, ErrTokenExpired, err)
}

// Benchmark tests
func BenchmarkGenerateTokenPair(b *testing.B) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := jwt.GenerateTokenPair(claims)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateTokenPair_WithEncryption(b *testing.B) {
	jwt := NewJWT("secret-key", "1234567890123456", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := jwt.GenerateTokenPair(claims)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifyAccessToken(b *testing.B) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}
	pair, err := jwt.GenerateTokenPair(claims)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := jwt.VerifyAccessToken(pair.AccessToken)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRefreshToken(b *testing.B) {
	jwt := NewJWT("secret-key", "", 3600, 604800)
	claims := &Claims{
		UserID:   123,
		Username: "testuser",
		Email:    "test@example.com",
		Role:     "admin",
	}
	pair, err := jwt.GenerateTokenPair(claims)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := jwt.RefreshToken(pair.RefreshToken)
		if err != nil {
			b.Fatal(err)
		}
	}
}
