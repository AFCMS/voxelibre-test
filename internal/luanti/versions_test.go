// SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
// SPDX-License-Identifier: LGPL-3.0-or-later

package luanti

import (
	"reflect"
	"strings"
	"testing"
)

func TestAvailableVersions(t *testing.T) {
	want := []string{"5.14.0", "5.15.2", "5.16.1"}
	versions := AvailableVersions()
	if !reflect.DeepEqual(versions, want) {
		t.Fatalf("AvailableVersions() = %#v, want %#v", versions, want)
	}

	versions[0] = "modified"
	if reflect.DeepEqual(AvailableVersions(), versions) {
		t.Fatal("AvailableVersions() exposed the catalog backing array")
	}
}

func TestSelectBuilds(t *testing.T) {
	tests := []struct {
		name    string
		version string
		all     bool
		kind    BuildKind
		want    []string
	}{
		{
			name:    "single version",
			version: "5.15.2",
			kind:    BuildKindAll,
			want:    []string{"luanti-5.15.2-server", "luanti-5.15.2-client"},
		},
		{
			name:    "single client",
			version: "5.16.1",
			kind:    BuildKindClient,
			want:    []string{"luanti-5.16.1-client"},
		},
		{
			name: "all servers",
			all:  true,
			kind: BuildKindServer,
			want: []string{"luanti-5.14.0-server", "luanti-5.15.2-server", "luanti-5.16.1-server"},
		},
		{
			name: "all builds",
			all:  true,
			kind: BuildKindAll,
			want: []string{
				"luanti-5.14.0-server", "luanti-5.14.0-client",
				"luanti-5.15.2-server", "luanti-5.15.2-client",
				"luanti-5.16.1-server", "luanti-5.16.1-client",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builds, err := SelectBuilds(test.version, test.all, test.kind)
			if err != nil {
				t.Fatal(err)
			}
			names := make([]string, len(builds))
			for index, build := range builds {
				names[index] = build.Name()
			}
			if !reflect.DeepEqual(names, test.want) {
				t.Fatalf("builds = %#v, want %#v", names, test.want)
			}
		})
	}
}

func TestSelectBuildsValidation(t *testing.T) {
	tests := []struct {
		version string
		all     bool
		kind    BuildKind
		want    string
	}{
		{kind: BuildKindAll, want: "required"},
		{version: "5.16.1", all: true, kind: BuildKindAll, want: "mutually exclusive"},
		{version: "1.0.0", kind: BuildKindAll, want: "unsupported Luanti version"},
		{all: true, kind: "desktop", want: "unsupported build kind"},
	}

	for _, test := range tests {
		_, err := SelectBuilds(test.version, test.all, test.kind)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("SelectBuilds(%q, %t, %q) = %v, want %q", test.version, test.all, test.kind, err, test.want)
		}
	}
}

func TestSupportedServerVersionsUseCatalogPaths(t *testing.T) {
	versions := SupportedServerVersions()
	if len(versions) != 3 {
		t.Fatalf("server versions = %d, want 3", len(versions))
	}
	for _, version := range versions {
		wantSuffix := "/luanti-" + version.Version + "-server/bin/luantiserver"
		if !strings.HasSuffix(version.Entrypoint, wantSuffix) {
			t.Fatalf("entrypoint %q does not end in %q", version.Entrypoint, wantSuffix)
		}
	}
}
