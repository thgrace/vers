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
const defaultEcosystemsBaseURL = "https://packages.ecosyste.ms/api/v1"

// VersionProvider identifies a package version metadata provider.
type VersionProvider string

const (
	// VersionProviderDepsDev fetches versions from deps.dev.
	VersionProviderDepsDev VersionProvider = "deps.dev"
	// VersionProviderEcosystems fetches versions from packages.ecosyste.ms.
	VersionProviderEcosystems VersionProvider = "ecosyste.ms"
)

// VersionFetchOption configures version fetching helpers.
type VersionFetchOption func(*versionFetchConfig)

// DepsDevOption is kept as an alias for backwards-compatible deps.dev helpers.
type DepsDevOption = VersionFetchOption

type versionFetchConfig struct {
	depsDevBaseURL     string
	ecosystemsBaseURL  string
	ecosystemsMailto   string
	ecosystemsRegistry string
	httpClient         *http.Client
}

// WithDepsDevBaseURL overrides the deps.dev API base URL.
//
// This is primarily useful for tests or proxies. The default is
// https://api.deps.dev/v3.
func WithDepsDevBaseURL(baseURL string) VersionFetchOption {
	return func(c *versionFetchConfig) {
		c.depsDevBaseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithEcosystemsBaseURL overrides the packages.ecosyste.ms API base URL.
//
// This is primarily useful for tests or mirrors. The default is
// https://packages.ecosyste.ms/api/v1.
func WithEcosystemsBaseURL(baseURL string) VersionFetchOption {
	return func(c *versionFetchConfig) {
		c.ecosystemsBaseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithEcosystemsMailto adds a mailto query parameter to ecosyste.ms requests.
//
// Ecosyste.ms uses this to place requests in its polite API pool.
func WithEcosystemsMailto(mailto string) VersionFetchOption {
	return func(c *versionFetchConfig) {
		c.ecosystemsMailto = strings.TrimSpace(mailto)
	}
}

// WithEcosystemsRegistry overrides the registry used for ecosyste.ms requests.
//
// If unset, the registry is inferred from the scheme.
func WithEcosystemsRegistry(registry string) VersionFetchOption {
	return func(c *versionFetchConfig) {
		c.ecosystemsRegistry = strings.TrimSpace(registry)
	}
}

// WithVersionHTTPClient overrides the HTTP client used by version fetch helpers.
func WithVersionHTTPClient(client *http.Client) VersionFetchOption {
	return func(c *versionFetchConfig) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithDepsDevHTTPClient overrides the HTTP client used by deps.dev helpers.
func WithDepsDevHTTPClient(client *http.Client) VersionFetchOption {
	return WithVersionHTTPClient(client)
}

// FetchVersions returns versions known by provider for packageName in scheme.
//
// Supported providers are VersionProviderDepsDev and VersionProviderEcosystems.
func FetchVersions(ctx context.Context, provider VersionProvider, scheme, packageName string, opts ...VersionFetchOption) ([]string, error) {
	switch provider {
	case VersionProviderDepsDev:
		return fetchDepsDevVersions(ctx, scheme, packageName, opts...)
	case VersionProviderEcosystems:
		return fetchEcosystemsVersions(ctx, scheme, packageName, opts...)
	default:
		return nil, fmt.Errorf("unsupported version provider %q", provider)
	}
}

// MatchingVersionsFromProvider fetches known package versions from provider and
// returns the versions that match constraint under scheme.
//
// If scheme is empty, constraint must be a vers URI and the provider scheme is
// derived from that URI. Otherwise, constraint is parsed as native package
// manager syntax for scheme.
func MatchingVersionsFromProvider(ctx context.Context, provider VersionProvider, packageName, constraint, scheme string, opts ...VersionFetchOption) ([]string, error) {
	versionScheme := scheme
	if versionScheme == "" {
		var err error
		versionScheme, err = schemeFromVersURI(constraint)
		if err != nil {
			return nil, err
		}
	}

	versions, err := FetchVersions(ctx, provider, versionScheme, packageName, opts...)
	if err != nil {
		return nil, err
	}
	return MatchingVersions(versions, constraint, scheme)
}

// DepsDevVersions returns the versions known by deps.dev for packageName in
// scheme. The package name must use the package ecosystem's native package name.
//
// Supported schemes are npm, pypi, gem/rubygems, maven, nuget, cargo, and
// go/golang.
func DepsDevVersions(ctx context.Context, scheme, packageName string, opts ...DepsDevOption) ([]string, error) {
	return FetchVersions(ctx, VersionProviderDepsDev, scheme, packageName, opts...)
}

// MatchingVersionsFromDepsDev fetches known package versions from deps.dev and
// returns the versions that match constraint under scheme.
//
// If scheme is empty, constraint must be a vers URI and the deps.dev system is
// derived from that URI. Otherwise, constraint is parsed as native package
// manager syntax for scheme.
func MatchingVersionsFromDepsDev(ctx context.Context, packageName, constraint, scheme string, opts ...DepsDevOption) ([]string, error) {
	return MatchingVersionsFromProvider(ctx, VersionProviderDepsDev, packageName, constraint, scheme, opts...)
}

// EcosystemsVersions returns the versions known by packages.ecosyste.ms for
// packageName in scheme. The package name must use the package ecosystem's
// native package name.
func EcosystemsVersions(ctx context.Context, scheme, packageName string, opts ...VersionFetchOption) ([]string, error) {
	return FetchVersions(ctx, VersionProviderEcosystems, scheme, packageName, opts...)
}

// MatchingVersionsFromEcosystems fetches known package versions from
// packages.ecosyste.ms and returns the versions that match constraint under
// scheme.
func MatchingVersionsFromEcosystems(ctx context.Context, packageName, constraint, scheme string, opts ...VersionFetchOption) ([]string, error) {
	return MatchingVersionsFromProvider(ctx, VersionProviderEcosystems, packageName, constraint, scheme, opts...)
}

func fetchDepsDevVersions(ctx context.Context, scheme, packageName string, opts ...VersionFetchOption) ([]string, error) {
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

	cfg := applyVersionFetchOptions(opts)
	endpoint := cfg.depsDevBaseURL + "/systems/" + escapeURLPathSegment(system) + "/packages/" + escapeURLPathSegment(packageName)
	var payload depsDevPackageResponse
	if err := fetchJSON(ctx, cfg.httpClient, endpoint, "deps.dev package versions", &payload); err != nil {
		return nil, err
	}

	versions := make([]string, 0, len(payload.Versions))
	for _, version := range payload.Versions {
		if version.VersionKey.Version != "" {
			versions = append(versions, version.VersionKey.Version)
		}
	}
	return versions, nil
}

func fetchEcosystemsVersions(ctx context.Context, scheme, packageName string, opts ...VersionFetchOption) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	cfg := applyVersionFetchOptions(opts)
	registry := cfg.ecosystemsRegistry
	if registry == "" {
		var err error
		registry, err = ecosystemsRegistry(scheme)
		if err != nil {
			return nil, err
		}
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return nil, fmt.Errorf("package name is required")
	}

	endpoint := cfg.ecosystemsBaseURL + "/registries/" + escapeURLPathSegment(registry) + "/packages/" + escapeURLPathSegment(packageName) + "/version_numbers"
	if cfg.ecosystemsMailto != "" {
		endpoint += "?mailto=" + url.QueryEscape(cfg.ecosystemsMailto)
	}

	var versions []string
	err := fetchJSON(ctx, cfg.httpClient, endpoint, "ecosyste.ms package versions", &versions)
	if err != nil {
		return nil, err
	}
	return versions, nil
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

func applyVersionFetchOptions(opts []VersionFetchOption) versionFetchConfig {
	cfg := versionFetchConfig{
		depsDevBaseURL:    defaultDepsDevBaseURL,
		ecosystemsBaseURL: defaultEcosystemsBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.depsDevBaseURL == "" {
		cfg.depsDevBaseURL = defaultDepsDevBaseURL
	}
	if cfg.ecosystemsBaseURL == "" {
		cfg.ecosystemsBaseURL = defaultEcosystemsBaseURL
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

func ecosystemsRegistry(scheme string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "npm":
		return "npmjs.org", nil
	case "pypi":
		return "pypi.org", nil
	case "gem", "rubygems":
		return "rubygems.org", nil
	case "maven":
		return "repo1.maven.org", nil
	case "nuget":
		return "nuget.org", nil
	case "cargo":
		return "crates.io", nil
	case "go", "golang":
		return "proxy.golang.org", nil
	case "composer", "packagist":
		return "packagist.org", nil
	default:
		return "", fmt.Errorf("unsupported ecosyste.ms scheme %q", scheme)
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

func fetchJSON(ctx context.Context, client *http.Client, endpoint, description string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create %s request: %w", description, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", description, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s request failed with status %d: %s", description, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode %s response: %w", description, err)
	}
	return nil
}

func escapeURLPathSegment(value string) string {
	escaped := url.PathEscape(value)
	return strings.ReplaceAll(escaped, "@", "%40")
}
