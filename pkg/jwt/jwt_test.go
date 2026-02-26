package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return key
}

func TestGenerateToken_Success(t *testing.T) {
	key := generateTestKey(t)

	tokenString, err := GenerateToken(key, 42, "johndoe", "admin", 24)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if tokenString == "" {
		t.Fatal("expected non-empty token string")
	}

	// Parse and verify the token
	token, err := gojwt.Parse(tokenString, func(token *gojwt.Token) (any, error) {
		if _, ok := token.Method.(*gojwt.SigningMethodRSA); !ok {
			t.Fatalf("unexpected signing method: %v", token.Header["alg"])
		}
		return &key.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	if !token.Valid {
		t.Fatal("expected valid token")
	}

	claims, ok := token.Claims.(gojwt.MapClaims)
	if !ok {
		t.Fatal("expected MapClaims")
	}

	// Verify claims
	if int(claims["user_id"].(float64)) != 42 {
		t.Errorf("expected user_id 42, got %v", claims["user_id"])
	}
	if claims["username"] != "johndoe" {
		t.Errorf("expected username 'johndoe', got %v", claims["username"])
	}
	if claims["role"] != "admin" {
		t.Errorf("expected role 'admin', got %v", claims["role"])
	}

	// Verify expiration is ~24h from now
	exp := time.Unix(int64(claims["exp"].(float64)), 0)
	expectedExp := time.Now().Add(24 * time.Hour)
	diff := expectedExp.Sub(exp)
	if diff < -1*time.Minute || diff > 1*time.Minute {
		t.Errorf("expected exp to be ~24h from now, got diff %v", diff)
	}

	// Verify iat is approximately now
	iat := time.Unix(int64(claims["iat"].(float64)), 0)
	iatDiff := time.Since(iat)
	if iatDiff < 0 || iatDiff > 1*time.Minute {
		t.Errorf("expected iat to be approximately now, got diff %v", iatDiff)
	}
}

func TestGenerateToken_NilKey(t *testing.T) {
	_, err := GenerateToken(nil, 1, "user", "user", 1)
	if err == nil {
		t.Fatal("expected error for nil private key")
	}
}

func TestGenerateToken_DifferentExpiry(t *testing.T) {
	key := generateTestKey(t)

	tokenString, err := GenerateToken(key, 1, "user", "user", 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	token, err := gojwt.Parse(tokenString, func(token *gojwt.Token) (any, error) {
		return &key.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	claims := token.Claims.(gojwt.MapClaims)
	exp := time.Unix(int64(claims["exp"].(float64)), 0)
	expectedExp := time.Now().Add(1 * time.Hour)
	diff := expectedExp.Sub(exp)
	if diff < -1*time.Minute || diff > 1*time.Minute {
		t.Errorf("expected exp to be ~1h from now, got diff %v", diff)
	}
}

func TestGenerateToken_WrongPublicKeyFails(t *testing.T) {
	key := generateTestKey(t)
	otherKey := generateTestKey(t)

	tokenString, err := GenerateToken(key, 1, "user", "user", 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Parsing with the wrong public key should fail
	_, err = gojwt.Parse(tokenString, func(token *gojwt.Token) (any, error) {
		return &otherKey.PublicKey, nil
	})
	if err == nil {
		t.Fatal("expected error when verifying with wrong public key")
	}
}

func TestGenerateToken_SigningMethodRS256(t *testing.T) {
	key := generateTestKey(t)

	tokenString, err := GenerateToken(key, 1, "user", "user", 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	parser := gojwt.NewParser()
	token, _, err := parser.ParseUnverified(tokenString, gojwt.MapClaims{})
	if err != nil {
		t.Fatalf("failed to parse unverified token: %v", err)
	}

	if token.Method.Alg() != "RS256" {
		t.Errorf("expected signing method RS256, got %s", token.Method.Alg())
	}
}

// ===========================================================================
// ValidateToken
// ===========================================================================

func TestValidateToken_Success(t *testing.T) {
	key := generateTestKey(t)

	tokenString, err := GenerateToken(key, 42, "johndoe", "admin", 24)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := ValidateToken(&key.PublicKey, tokenString)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if claims.UserID != 42 {
		t.Errorf("expected user_id 42, got %d", claims.UserID)
	}
	if claims.Username != "johndoe" {
		t.Errorf("expected username 'johndoe', got %q", claims.Username)
	}
	if claims.Role != "admin" {
		t.Errorf("expected role 'admin', got %q", claims.Role)
	}
}

func TestValidateToken_NilPublicKey(t *testing.T) {
	key := generateTestKey(t)
	tokenString, _ := GenerateToken(key, 1, "user", "user", 1)

	_, err := ValidateToken(nil, tokenString)
	if err == nil {
		t.Fatal("expected error for nil public key")
	}
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	key := generateTestKey(t)

	// Generate a token that expired 1 hour ago
	tokenString, err := GenerateToken(key, 1, "user", "user", -1)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = ValidateToken(&key.PublicKey, tokenString)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateToken_WrongKey(t *testing.T) {
	key := generateTestKey(t)
	otherKey := generateTestKey(t)

	tokenString, _ := GenerateToken(key, 1, "user", "user", 1)

	_, err := ValidateToken(&otherKey.PublicKey, tokenString)
	if err == nil {
		t.Fatal("expected error when validating with wrong public key")
	}
}

func TestValidateToken_InvalidTokenString(t *testing.T) {
	key := generateTestKey(t)

	_, err := ValidateToken(&key.PublicKey, "not.a.valid.token")
	if err == nil {
		t.Fatal("expected error for invalid token string")
	}
}
