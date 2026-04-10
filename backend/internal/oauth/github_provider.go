package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"lazyops-server/internal/config"
	"lazyops-server/internal/service"
)

const (
	githubAuthorizationURL = "https://github.com/login/oauth/authorize"
	githubTokenURL         = "https://github.com/login/oauth/access_token"
	githubUserURL          = "https://api.github.com/user"
	githubUserEmailsURL    = "https://api.github.com/user/emails"
)

type GitHubProvider struct {
	cfg        config.GitHubOAuthConfig
	httpClient *http.Client
}

func NewGitHubProvider(cfg config.GitHubOAuthConfig, httpClient *http.Client) *GitHubProvider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &GitHubProvider{
		cfg:        cfg,
		httpClient: httpClient,
	}
}

func (p *GitHubProvider) AuthorizationURL(state string) string {
	query := url.Values{}
	query.Set("client_id", p.cfg.ClientID)
	query.Set("redirect_uri", p.cfg.CallbackURL)
	query.Set("scope", strings.Join(p.oauthScopes(), " "))
	query.Set("state", state)

	return githubAuthorizationURL + "?" + query.Encode()
}

func (p *GitHubProvider) ExchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", p.cfg.ClientID)
	form.Set("client_secret", p.cfg.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", p.cfg.CallbackURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		return "", err
	}

	var jsonPayload struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &jsonPayload)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if strings.TrimSpace(jsonPayload.ErrorDescription) != "" || strings.TrimSpace(jsonPayload.Error) != "" {
			return "", fmt.Errorf("github token exchange failed: %s (%s)", strings.TrimSpace(jsonPayload.Error), strings.TrimSpace(jsonPayload.ErrorDescription))
		}
		parsed, _ := url.ParseQuery(string(body))
		if errDesc := strings.TrimSpace(parsed.Get("error_description")); errDesc != "" {
			return "", fmt.Errorf("github token exchange failed: %s (%s)", strings.TrimSpace(parsed.Get("error")), errDesc)
		}
		raw := strings.TrimSpace(string(body))
		if raw != "" {
			return "", fmt.Errorf("github token exchange failed (status=%d): %s", resp.StatusCode, raw)
		}
		return "", fmt.Errorf("github token exchange failed (status=%d)", resp.StatusCode)
	}

	accessToken := strings.TrimSpace(jsonPayload.AccessToken)
	if accessToken == "" {
		parsed, _ := url.ParseQuery(string(body))
		accessToken = strings.TrimSpace(parsed.Get("access_token"))
	}
	if accessToken == "" {
		if strings.TrimSpace(jsonPayload.ErrorDescription) != "" || strings.TrimSpace(jsonPayload.Error) != "" {
			return "", fmt.Errorf("github access token missing: %s (%s)", strings.TrimSpace(jsonPayload.Error), strings.TrimSpace(jsonPayload.ErrorDescription))
		}
		parsed, _ := url.ParseQuery(string(body))
		if errDesc := strings.TrimSpace(parsed.Get("error_description")); errDesc != "" {
			return "", fmt.Errorf("github access token missing: %s (%s)", strings.TrimSpace(parsed.Get("error")), errDesc)
		}
		return "", errors.New("github access token missing")
	}

	return accessToken, nil
}

func (p *GitHubProvider) FetchIdentity(ctx context.Context, accessToken string) (*service.GitHubIdentity, error) {
	profileReq, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserURL, nil)
	if err != nil {
		return nil, err
	}
	profileReq.Header.Set("Authorization", "Bearer "+accessToken)
	profileReq.Header.Set("Accept", "application/vnd.github+json")
	profileReq.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	profileResp, err := p.httpClient.Do(profileReq)
	if err != nil {
		return nil, err
	}
	defer profileResp.Body.Close()

	if profileResp.StatusCode < http.StatusOK || profileResp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New("github user fetch failed")
	}

	var profile struct {
		ID        int64   `json:"id"`
		Login     string  `json:"login"`
		Name      string  `json:"name"`
		Email     *string `json:"email"`
		AvatarURL string  `json:"avatar_url"`
	}
	if err := json.NewDecoder(profileResp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	if profile.ID == 0 || strings.TrimSpace(profile.Login) == "" {
		return nil, errors.New("github subject missing")
	}

	email := ""
	if profile.Email != nil {
		email = strings.TrimSpace(*profile.Email)
	}
	if email == "" {
		resolvedEmail, err := p.fetchPrimaryEmail(ctx, accessToken)
		if err == nil {
			email = resolvedEmail
		}
	}
	if email == "" {
		email = fallbackGitHubEmail(profile.ID)
	}

	return &service.GitHubIdentity{
		Subject:   strconv.FormatInt(profile.ID, 10),
		Login:     profile.Login,
		Email:     email,
		Name:      profile.Name,
		AvatarURL: profile.AvatarURL,
	}, nil
}

func (p *GitHubProvider) fetchPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserEmailsURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", errors.New("github emails fetch failed")
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, email := range emails {
		if email.Primary && email.Verified && strings.TrimSpace(email.Email) != "" {
			return strings.TrimSpace(email.Email), nil
		}
	}
	for _, email := range emails {
		if email.Verified && strings.TrimSpace(email.Email) != "" {
			return strings.TrimSpace(email.Email), nil
		}
	}

	return "", fmt.Errorf("github verified email missing")
}

func fallbackGitHubEmail(subjectID int64) string {
	if subjectID <= 0 {
		return "github-unknown@users.noreply.github.com"
	}
	return fmt.Sprintf("github-%d@users.noreply.github.com", subjectID)
}

func (p *GitHubProvider) oauthScopes() []string {
	if len(p.cfg.Scopes) == 0 {
		return []string{"read:user", "user:email", "read:org", "repo"}
	}

	normalized := make([]string, 0, len(p.cfg.Scopes))
	for _, scope := range p.cfg.Scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	if len(normalized) == 0 {
		return []string{"read:user", "user:email", "read:org", "repo"}
	}
	return normalized
}
