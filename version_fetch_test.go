package vers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestDepsDevVersions(t *testing.T) {
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

	got, err := DepsDevVersions(
		context.Background(),
		"npm",
		"@scope/pkg",
		WithDepsDevBaseURL(server.URL+"/v3"),
		serverHTTPClient(server),
	)
	if err != nil {
		t.Fatalf("DepsDevVersions() error = %v", err)
	}
	want := []string{"1.0.0", "1.5.0", "2.0.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DepsDevVersions() = %#v, want %#v", got, want)
	}
}

func TestMatchingVersionsFromDepsDev(t *testing.T) {
	server := depsDevVersionServer(t, "PYPI", "demo", []string{"1.4.1", "1.4.2", "1.4.9", "1.5.0"})
	defer server.Close()

	got, err := MatchingVersionsFromDepsDev(
		context.Background(),
		"demo",
		"~=1.4.2",
		"pypi",
		WithDepsDevBaseURL(server.URL+"/v3"),
		serverHTTPClient(server),
	)
	if err != nil {
		t.Fatalf("MatchingVersionsFromDepsDev() error = %v", err)
	}
	want := []string{"1.4.2", "1.4.9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchingVersionsFromDepsDev() = %#v, want %#v", got, want)
	}
}

func TestMatchingVersionsFromDepsDevVersURI(t *testing.T) {
	server := depsDevVersionServer(t, "NPM", "demo", []string{"1.0.0", "1.4.0", "1.5.0", "2.0.0"})
	defer server.Close()

	got, err := MatchingVersionsFromDepsDev(
		context.Background(),
		"demo",
		"vers:npm/>=1.0.0|<2.0.0|!=1.5.0",
		"",
		WithDepsDevBaseURL(server.URL+"/v3"),
		serverHTTPClient(server),
	)
	if err != nil {
		t.Fatalf("MatchingVersionsFromDepsDev() error = %v", err)
	}
	want := []string{"1.0.0", "1.4.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchingVersionsFromDepsDev() = %#v, want %#v", got, want)
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
	)
	if err != nil {
		t.Fatalf("FetchVersions() error = %v", err)
	}
	want := []string{"4.52.4", "4.53.0", "4.57.3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FetchVersions() = %#v, want %#v", got, want)
	}
}

func TestEcosystemsVersionsCustomRegistry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/api/v1/registries/custom.example/packages/%40scope%2Fpkg/version_numbers"; got != want {
			t.Errorf("path = %s, want %s", got, want)
		}
		fmt.Fprint(w, `["1.0.0"]`)
	}))
	defer server.Close()

	got, err := EcosystemsVersions(
		context.Background(),
		"custom",
		"@scope/pkg",
		WithEcosystemsBaseURL(server.URL+"/api/v1"),
		WithEcosystemsRegistry("custom.example"),
		WithVersionHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("EcosystemsVersions() error = %v", err)
	}
	want := []string{"1.0.0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EcosystemsVersions() = %#v, want %#v", got, want)
	}
}

func TestMatchingVersionsFromEcosystems(t *testing.T) {
	server := ecosystemsVersionServer(t, "pypi.org", "transformers", []string{"4.52.4", "4.53.0", "4.57.3"})
	defer server.Close()

	got, err := MatchingVersionsFromEcosystems(
		context.Background(),
		"transformers",
		"<4.53.0",
		"pypi",
		WithEcosystemsBaseURL(server.URL+"/api/v1"),
		WithVersionHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("MatchingVersionsFromEcosystems() error = %v", err)
	}
	want := []string{"4.52.4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MatchingVersionsFromEcosystems() = %#v, want %#v", got, want)
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

func TestDepsDevVersionsUnsupportedScheme(t *testing.T) {
	_, err := DepsDevVersions(context.Background(), "rpm", "demo")
	if err == nil {
		t.Fatal("DepsDevVersions() error = nil, want non-nil")
	}
}

func TestFetchVersionsUnsupportedProvider(t *testing.T) {
	_, err := FetchVersions(context.Background(), VersionProvider("unknown"), "npm", "demo")
	if err == nil {
		t.Fatal("FetchVersions() error = nil, want non-nil")
	}
}

func TestMatchingVersionsFromDepsDevRequiresSchemeForNativeConstraint(t *testing.T) {
	_, err := MatchingVersionsFromDepsDev(context.Background(), "demo", "^1.0.0", "")
	if err == nil {
		t.Fatal("MatchingVersionsFromDepsDev() error = nil, want non-nil")
	}
}

func TestDepsDevVersionsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := DepsDevVersions(
		context.Background(),
		"npm",
		"missing",
		WithDepsDevBaseURL(server.URL+"/v3"),
		serverHTTPClient(server),
	)
	if err == nil {
		t.Fatal("DepsDevVersions() error = nil, want non-nil")
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

func serverHTTPClient(server *httptest.Server) DepsDevOption {
	return WithDepsDevHTTPClient(server.Client())
}
