package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"lazyops-server/internal/config"
	"lazyops-server/internal/service"
)

const (
	googleAuthorizationURL = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL         = "https://oauth2.googleapis.com/token"
	googleUserInfoURL      = "https://openidconnect.googleapis.com/v1/userinfo"
)

type GoogleProvider struct {
	cfg        config.GoogleOAuthConfig
	httpClient *http.Client
}

func NewGoogleProvider(cfg config.GoogleOAuthConfig, httpClient *http.Client) *GoogleProvider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &GoogleProvider{
		cfg:        cfg,
		httpClient: httpClient,
	}
}

func (p *GoogleProvider) AuthorizationURL(state string) string {
	query := url.Values{}
	query.Set("client_id", p.cfg.ClientID)
	query.Set("redirect_uri", p.cfg.CallbackURL)
	query.Set("response_type", "code")
	query.Set("scope", "openid email profile")
	query.Set("state", state)
	query.Set("access_type", "online")
	query.Set("include_granted_scopes", "true")
	query.Set("prompt", "select_account")

	return googleAuthorizationURL + "?" + query.Encode()
}

func (p *GoogleProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", p.cfg.ClientID)
	form.Set("client_secret", p.cfg.ClientSecret)
	form.Set("redirect_uri", p.cfg.CallbackURL)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", errors.New("google token exchange failed")
	}

	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", errors.New("google access token missing")
	}

	return payload.AccessToken, nil
}

func (p *GoogleProvider) FetchIdentity(ctx context.Context, accessToken string) (*service.GoogleIdentity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New("google userinfo fetch failed")
	}

	var payload struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return &service.GoogleIdentity{
		Subject:       payload.Sub,
		Email:         payload.Email,
		EmailVerified: payload.EmailVerified,
		Name:          payload.Name,
		AvatarURL:     payload.Picture,
	}, nil
}
