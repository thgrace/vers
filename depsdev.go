package vers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultDepsDevBaseURL = "https://api.deps.dev/v3"

// DepsDevOption configures deps.dev-backed helpers.
type DepsDevOption func(*depsDevConfig)

type depsDevConfig struct {
	baseURL    string
	httpClient *http.Client
}

// WithDepsDevBaseURL overrides the deps.dev API base URL.
//
// This is primarily useful for tests or proxies. The default is
// https://api.deps.dev/v3.
func WithDepsDevBaseURL(baseURL string) DepsDevOption {
	return func(c *depsDevConfig) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithDepsDevHTTPClient overrides the HTTP client used by deps.dev helpers.
func WithDepsDevHTTPClient(client *http.Client) DepsDevOption {
	return func(c *depsDevConfig) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// DepsDevVersions returns the versions known by deps.dev for packageName in
// scheme. The package name must use the package ecosystem's native package name.
//
// Supported schemes are npm, pypi, gem/rubygems, maven, nuget, cargo, and
// go/golang.
func DepsDevVersions(ctx context.Context, scheme, packageName string, opts ...DepsDevOption) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	system, err := depsDevSystem(scheme)
	if err != nil {
		return nil, err
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return nil, fmt.Errorf("package name is required")
	}

	cfg := applyDepsDevOptions(opts)
	endpoint := cfg.baseURL + "/systems/" + escapeDepsDevPathSegment(system) + "/packages/" + escapeDepsDevPathSegment(packageName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create deps.dev request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := cfg.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch deps.dev package versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("deps.dev package versions request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload depsDevPackageResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode deps.dev package response: %w", err)
	}

	versions := make([]string, 0, len(payload.Versions))
	for _, version := range payload.Versions {
		if version.VersionKey.Version != "" {
			versions = append(versions, version.VersionKey.Version)
		}
	}
	return versions, nil
}

// MatchingVersionsFromDepsDev fetches known package versions from deps.dev and
// returns the versions that match constraint under scheme.
//
// If scheme is empty, constraint must be a vers URI and the deps.dev system is
// derived from that URI. Otherwise, constraint is parsed as native package
// manager syntax for scheme.
func MatchingVersionsFromDepsDev(ctx context.Context, packageName, constraint, scheme string, opts ...DepsDevOption) ([]string, error) {
	versionScheme := scheme
	if versionScheme == "" {
		var err error
		versionScheme, err = schemeFromVersURI(constraint)
		if err != nil {
			return nil, err
		}
	}

	versions, err := DepsDevVersions(ctx, versionScheme, packageName, opts...)
	if err != nil {
		return nil, err
	}
	return MatchingVersions(versions, constraint, scheme)
}

type depsDevPackageResponse struct {
	Versions []depsDevPackageVersion `json:"versions"`
}

type depsDevPackageVersion struct {
	VersionKey depsDevVersionKey `json:"versionKey"`
}

type depsDevVersionKey struct {
	Version string `json:"version"`
}

func applyDepsDevOptions(opts []DepsDevOption) depsDevConfig {
	cfg := depsDevConfig{
		baseURL: defaultDepsDevBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.baseURL == "" {
		cfg.baseURL = defaultDepsDevBaseURL
	}
	return cfg
}

func depsDevSystem(scheme string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "npm":
		return "NPM", nil
	case "pypi":
		return "PYPI", nil
	case "gem", "rubygems":
		return "RUBYGEMS", nil
	case "maven":
		return "MAVEN", nil
	case "nuget":
		return "NUGET", nil
	case "cargo":
		return "CARGO", nil
	case "go", "golang":
		return "GO", nil
	default:
		return "", fmt.Errorf("unsupported deps.dev scheme %q", scheme)
	}
}

func schemeFromVersURI(constraint string) (string, error) {
	trimmed := strings.TrimSpace(constraint)
	if !strings.HasPrefix(trimmed, "vers:") {
		return "", fmt.Errorf("scheme is required when constraint is not a vers URI")
	}
	rest := strings.TrimPrefix(trimmed, "vers:")
	scheme, _, ok := strings.Cut(rest, "/")
	if !ok || strings.TrimSpace(scheme) == "" {
		return "", fmt.Errorf("invalid vers URI: %s", constraint)
	}
	return scheme, nil
}

func escapeDepsDevPathSegment(value string) string {
	escaped := url.PathEscape(value)
	return strings.ReplaceAll(escaped, "@", "%40")
}
