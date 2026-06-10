package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestFetchVersionsDepsDev(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if got, want := r.URL.EscapedPath(), "/v3/systems/NPM/packages/%40scope%2Fpkg"; got != want {
			t.Errorf("path = %s, want %s", got, want)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		if got := r.Header.Get("User-Agent"); got != defaultUserAgent {
			t.Errorf("User-Agent = %q, want %q", got, defaultUserAgent)
		}
		fmt.Fprint(w, `{
			"versions": [
				{"versionKey": {"system": "NPM", "name": "@scope/pkg", "version": "1.0.0"}},
				{"versionKey": {"system": "NPM", "name": "@scope/pkg", "version": "1.5.0"}},
				{"versionKey": {"system": "NPM", "name": "@scope/pkg", "version": ""}},
				{"versionKey": {"system": "NPM", "name": "@scope/pkg", "version": "2.0.0"}}
			]
		}`)
	}))
	defer server.Close()

	got, err := FetchVersions(
		context.Background(),
		VersionProviderDepsDev,
		"npm",
		"@scope/pkg",
		WithDepsDevBaseURL(server.URL+"/v3"),
		serverHTTPClient(server),
	)
	if err != nil {
		t.Fatalf("FetchVersions() error = %v", err)
	}
	want := []string{"1.0.0", "1.5.0", "2.0.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FetchVersions() = %#v, want %#v", got, want)
	}
}

func TestMatchingVersionsFromProviderDepsDev(t *testing.T) {
	server := depsDevVersionServer(t, "PYPI", "demo", []string{"1.4.1", "1.4.2", "1.4.9", "1.5.0"})
	defer server.Close()

	got, err := MatchingVersionsFromProvider(
		context.Background(),
		VersionProviderDepsDev,
		"demo",
		"~=1.4.2",
		"pypi",
		WithDepsDevBaseURL(server.URL+"/v3"),
		serverHTTPClient(server),
	)
	if err != nil {
		t.Fatalf("MatchingVersionsFromProvider() error = %v", err)
	}
	want := []string{"1.4.2", "1.4.9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchingVersionsFromProvider() = %#v, want %#v", got, want)
	}
}

func TestMatchingVersionsFromProviderDepsDevVersURI(t *testing.T) {
	server := depsDevVersionServer(t, "NPM", "demo", []string{"1.0.0", "1.4.0", "1.5.0", "2.0.0"})
	defer server.Close()

	got, err := MatchingVersionsFromProvider(
		context.Background(),
		VersionProviderDepsDev,
		"demo",
		"vers:npm/>=1.0.0|<2.0.0|!=1.5.0",
		"",
		WithDepsDevBaseURL(server.URL+"/v3"),
		serverHTTPClient(server),
	)
	if err != nil {
		t.Fatalf("MatchingVersionsFromProvider() error = %v", err)
	}
	want := []string{"1.0.0", "1.4.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchingVersionsFromProvider() = %#v, want %#v", got, want)
	}
}

func TestFetchVersionsEcosystems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/v1/registries/pypi.org/packages/transformers/version_numbers"; got != want {
			t.Errorf("path = %s, want %s", got, want)
		}
		if got, want := r.URL.Query().Get("mailto"), "maintainer@example.com"; got != want {
			t.Errorf("mailto = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("User-Agent"), "custom-client/1.0"; got != want {
			t.Errorf("User-Agent = %q, want %q", got, want)
		}
		fmt.Fprint(w, `["4.52.4","4.53.0","4.57.3"]`)
	}))
	defer server.Close()

	got, err := FetchVersions(
		context.Background(),
		VersionProviderEcosystems,
		"pypi",
		"transformers",
		WithEcosystemsBaseURL(server.URL+"/api/v1"),
		WithEcosystemsMailto("maintainer@example.com"),
		WithVersionHTTPClient(server.Client()),
		WithVersionUserAgent("custom-client/1.0"),
	)
	if err != nil {
		t.Fatalf("FetchVersions() error = %v", err)
	}
	want := []string{"4.52.4", "4.53.0", "4.57.3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FetchVersions() = %#v, want %#v", got, want)
	}
}

func TestFetchVersionsEcosystemsCustomRegistry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/v1/registries/custom.example/packages/%40scope%2Fpkg/version_numbers"; got != want {
			t.Errorf("path = %s, want %s", got, want)
		}
		fmt.Fprint(w, `["1.0.0"]`)
	}))
	defer server.Close()

	got, err := FetchVersions(
		context.Background(),
		VersionProviderEcosystems,
		"custom",
		"@scope/pkg",
		WithEcosystemsBaseURL(server.URL+"/api/v1"),
		WithEcosystemsRegistry("custom.example"),
		WithVersionHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("FetchVersions() error = %v", err)
	}
	want := []string{"1.0.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FetchVersions() = %#v, want %#v", got, want)
	}
}

func TestMatchingVersionsFromProviderEcosystems(t *testing.T) {
	server := ecosystemsVersionServer(t, "pypi.org", "transformers", []string{"4.52.4", "4.53.0", "4.57.3"})
	defer server.Close()

	got, err := MatchingVersionsFromProvider(
		context.Background(),
		VersionProviderEcosystems,
		"transformers",
		"<4.53.0",
		"pypi",
		WithEcosystemsBaseURL(server.URL+"/api/v1"),
		WithVersionHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("MatchingVersionsFromProvider() error = %v", err)
	}
	want := []string{"4.52.4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchingVersionsFromProvider() = %#v, want %#v", got, want)
	}
}

func TestMatchingVersionsFromProviderEcosystemsVersURI(t *testing.T) {
	server := ecosystemsVersionServer(t, "npmjs.org", "demo", []string{"1.0.0", "1.4.0", "1.5.0", "2.0.0"})
	defer server.Close()

	got, err := MatchingVersionsFromProvider(
		context.Background(),
		VersionProviderEcosystems,
		"demo",
		"vers:npm/>=1.0.0|<2.0.0|!=1.5.0",
		"",
		WithEcosystemsBaseURL(server.URL+"/api/v1"),
		WithVersionHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("MatchingVersionsFromProvider() error = %v", err)
	}
	want := []string{"1.0.0", "1.4.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchingVersionsFromProvider() = %#v, want %#v", got, want)
	}
}

func TestFetchVersionsDepsDevUnsupportedScheme(t *testing.T) {
	_, err := FetchVersions(context.Background(), VersionProviderDepsDev, "rpm", "demo")
	if err == nil {
		t.Fatal("FetchVersions() error = nil, want non-nil")
	}
}

func TestFetchVersionsUnsupportedProvider(t *testing.T) {
	_, err := FetchVersions(context.Background(), VersionProvider("unknown"), "npm", "demo")
	if err == nil {
		t.Fatal("FetchVersions() error = nil, want non-nil")
	}
}

func TestMatchingVersionsFromProviderRequiresSchemeForNativeConstraint(t *testing.T) {
	_, err := MatchingVersionsFromProvider(context.Background(), VersionProviderDepsDev, "demo", "^1.0.0", "")
	if err == nil {
		t.Fatal("MatchingVersionsFromProvider() error = nil, want non-nil")
	}
}

func TestFetchVersionsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := FetchVersions(
		context.Background(),
		VersionProviderDepsDev,
		"npm",
		"missing",
		WithDepsDevBaseURL(server.URL+"/v3"),
		serverHTTPClient(server),
	)
	if err == nil {
		t.Fatal("FetchVersions() error = nil, want non-nil")
	}
}

func TestFetchVersionsSuccessBodyTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Repeat(" ", maxSuccessResponseBodyBytes+1))
	}))
	defer server.Close()

	_, err := FetchVersions(
		context.Background(),
		VersionProviderDepsDev,
		"npm",
		"oversized",
		WithDepsDevBaseURL(server.URL+"/v3"),
		serverHTTPClient(server),
	)
	if err == nil {
		t.Fatal("FetchVersions() error = nil, want non-nil")
	}
	if got, want := err.Error(), "exceeds"; !strings.Contains(got, want) {
		t.Fatalf("FetchVersions() error = %q, want substring %q", got, want)
	}
}

func depsDevVersionServer(t *testing.T, system, name string, versions []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/v3/systems/"+system+"/packages/"+name; got != want {
			t.Errorf("path = %s, want %s", got, want)
		}
		fmt.Fprint(w, `{"versions":[`)
		for i, version := range versions {
			if i > 0 {
				fmt.Fprint(w, `,`)
			}
			fmt.Fprintf(w, `{"versionKey":{"system":%q,"name":%q,"version":%q}}`, system, name, version)
		}
		fmt.Fprint(w, `]}`)
	}))
}

func ecosystemsVersionServer(t *testing.T, registry, name string, versions []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/v1/registries/"+registry+"/packages/"+name+"/version_numbers"; got != want {
			t.Errorf("path = %s, want %s", got, want)
		}
		fmt.Fprint(w, `[`)
		for i, version := range versions {
			if i > 0 {
				fmt.Fprint(w, `,`)
			}
			fmt.Fprintf(w, `%q`, version)
		}
		fmt.Fprint(w, `]`)
	}))
}

func serverHTTPClient(server *httptest.Server) VersionFetchOption {
	return WithVersionHTTPClient(server.Client())
}
