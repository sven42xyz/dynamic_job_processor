package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"djp.chapter42.de/a/data"
)

type AuthProvider interface {
	GetAuthHeader() (string, error)
}

type BasicAuth struct {
	Username string
	Password string
}

func (b *BasicAuth) GetAuthHeader() (string, error) {
	encoded := base64.StdEncoding.EncodeToString([]byte(b.Username + ":" + b.Password))
	return "Basic " + encoded, nil
}

type BearerAuth struct {
	Token string
}

func (b *BearerAuth) GetAuthHeader() (string, error) {
	return "Bearer " + b.Token, nil
}

type OAuth2Auth struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	RefreshToken string

	accessToken string
	expiresAt   time.Time
	mu          sync.Mutex
}

func (o *OAuth2Auth) GetAuthHeader() (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if time.Now().Before(o.expiresAt) && o.accessToken != "" {
		return "Bearer " + o.accessToken, nil
	}
	return o.refreshAccessToken()
}

func (o *OAuth2Auth) refreshAccessToken() (string, error) {
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", o.RefreshToken)
	values.Set("client_id", o.ClientID)
	values.Set("client_secret", o.ClientSecret)

	resp, err := http.PostForm(o.TokenURL, values)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token error: %s", body)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("token parse error: %w", err)
	}

	o.accessToken = tokenResp.AccessToken
	o.expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn-10) * time.Second)

	return "Bearer " + o.accessToken, nil
}

func BuildAuthProvider(cfg data.AuthConfig) (AuthProvider, error) {
	switch strings.ToLower(cfg.Type) {
	case "basic":
		return &BasicAuth{
			Username: cfg.Username,
			Password: cfg.Password,
		}, nil
	case "bearer":
		return &BearerAuth{
			Token: cfg.Token,
		}, nil
	case "oauth2":
		return &OAuth2Auth{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			TokenURL:     cfg.TokenURL,
			RefreshToken: cfg.RefreshToken,
		}, nil
	default:
		return nil, errors.New("unbekannter Auth-Typ: " + cfg.Type)
	}
}
