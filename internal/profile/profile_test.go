package profile

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestProfileValidation(t *testing.T) {
	valid := "version: 1\nname: test\nroutes:\n  - hosts: [example.com]\n    path: /x\n    methods: [GET]\n    status: 200\n    body_file: body\n"
	tests := []struct {
		name, yaml string
		files      fstest.MapFS
		wantErr    bool
	}{
		{"valid", valid, fstest.MapFS{"test/profile.yaml": {Data: []byte(valid)}, "test/body": {Data: []byte("ok")}}, false},
		{"version", "version: 2\nname: test\nroutes: []\n", fstest.MapFS{"test/profile.yaml": {Data: []byte("version: 2\nname: test\nroutes: []\n")}}, true},
		{"wildcard", "version: 1\nname: test\nroutes:\n  - hosts: [bad*.example.com]\n    path: /x\n    methods: [GET]\n    status: 200\n    body_file: body\n", nil, true},
		{"path", "version: 1\nname: test\nroutes:\n  - hosts: [example.com]\n    path: x\n    methods: [GET]\n    status: 200\n    body_file: body\n", nil, true},
		{"method", "version: 1\nname: test\nroutes:\n  - hosts: [example.com]\n    path: /x\n    methods: [PUT]\n    status: 200\n    body_file: body\n", nil, true},
		{"body traversal", "version: 1\nname: test\nroutes:\n  - hosts: [example.com]\n    path: /x\n    methods: [GET]\n    status: 200\n    body_file: ../body\n", nil, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := test.files
			if files == nil {
				files = fstest.MapFS{"test/profile.yaml": {Data: []byte(test.yaml)}, "test/body": {Data: []byte("ok")}}
			}
			_, err := loadFS(fs.FS(files))
			if (err != nil) != test.wantErr {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestRouterResults(t *testing.T) {
	items := map[string]Profile{"test": {Name: "test", Routes: []Route{{Profile: "test", Hosts: []string{"example.com", "*.wild.example"}, Path: "/x", Methods: []string{"GET"}}}}}
	router, err := NewRouter(items, []string{"test"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		host, method, path string
		want               Result
	}{
		{"example.com", "GET", "/x", Matched}, {"example.com", "POST", "/x", MethodNotAllowed}, {"example.com", "GET", "/other", UnknownPath}, {"badexample.com", "GET", "/x", UnknownHost}, {"wild.example", "GET", "/x", UnknownHost}, {"one.wild.example", "GET", "/x", Matched},
	}
	for _, test := range tests {
		_, result := router.Match(test.host, test.method, test.path)
		if result != test.want {
			t.Fatalf("%s got %s", test.host, result)
		}
	}
}

func TestOperatorOverride(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "adshield")
	if err := os.Mkdir(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	data := []byte("version: 1\nname: adshield\nroutes:\n  - hosts: [html-load.com]\n    path: /override\n    methods: [GET]\n    status: 200\n    body_file: body\n")
	if err := os.WriteFile(filepath.Join(profileDir, "profile.yaml"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "body"), []byte("override"), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if items["adshield"].Routes[0].Path != "/override" {
		t.Fatal("bundled profile was not overridden")
	}
}
