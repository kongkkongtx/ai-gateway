// Package user provides user authentication, JWT token management,
// and password hashing for the AI Gateway.
package user

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// User represents a gateway user account.
type User struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Role         string    `json:"role"`
	Team         string    `json:"team,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// JWTClaims holds the standard and custom claims for a JWT.
type JWTClaims struct {
	Sub  string `json:"sub"`
	Role string `json:"role"`
	Team string `json:"team"`
	Iat  int64  `json:"iat"`
	Exp  int64  `json:"exp"`
}

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrCannotDeleteSelf   = errors.New("cannot delete your own account")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
)

var JWTSecret = "ai-gateway-jwt-secret-change-me"
const TokenExpiry = 24 * time.Hour

// HashPassword creates a salted SHA-256 hash. Format: "salt:hash" (base64).
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", ErrWeakPassword
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(password))
	hash := h.Sum(nil)
	return base64.RawStdEncoding.EncodeToString(salt) + ":" +
		base64.RawStdEncoding.EncodeToString(hash), nil
}

// CheckPassword verifies a password against a stored hash.
func CheckPassword(password, stored string) bool {
	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(password))
	actualHash := h.Sum(nil)
	return hmac.Equal(expectedHash, actualHash)
}

// GenerateToken creates a signed JWT token for a user.
func GenerateToken(u *User) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		Sub:  u.Username,
		Role: u.Role,
		Team: u.Team,
		Iat:  now.Unix(),
		Exp:  now.Add(TokenExpiry).Unix(),
	}
	return signJWT(claims, []byte(JWTSecret))
}

// ValidateToken parses and verifies a JWT token string.
func ValidateToken(tokenStr string) (*JWTClaims, error) {
	claims, err := parseJWT(tokenStr, []byte(JWTSecret))
	if err != nil {
		return nil, ErrInvalidToken
	}
	if time.Now().Unix() > claims.Exp {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func signJWT(claims JWTClaims, secret []byte) (string, error) {
	header := base64URLEncode(toJSON(map[string]string{"alg": "HS256", "typ": "JWT"}))
	payload := base64URLEncode(toJSON(claims))
	toSign := header + "." + payload
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(toSign))
	signature := base64URLEncode(string(mac.Sum(nil)))
	return header + "." + payload + "." + signature, nil
}

func parseJWT(tokenStr string, secret []byte) (*JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid jwt format")
	}
	toVerify := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(toVerify))
	expectedSig := base64URLEncode(string(mac.Sum(nil)))
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, errors.New("invalid signature")
	}
	payloadBytes, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var claims JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}
	return &claims, nil
}

// GenerateAPIKey creates a random 48-char API key with "sk-" prefix.
func GenerateAPIKey() (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 48)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		b[i] = chars[n.Int64()]
	}
	return "sk-" + string(b), nil
}

func base64URLEncode(data string) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString([]byte(data)), "=")
}

func base64URLDecode(data string) ([]byte, error) {
	if m := len(data) % 4; m != 0 {
		data += strings.Repeat("=", 4-m)
	}
	return base64.URLEncoding.DecodeString(data)
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
