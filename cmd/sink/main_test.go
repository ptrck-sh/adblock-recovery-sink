package main

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/pki"
)

func TestEffectiveHosts(t *testing.T) {
	directory := t.TempDir()
	if err := pki.Init(pki.InitOptions{Hosts: []string{"allowed.example"}, OutDir: directory}); err != nil {
		t.Fatal(err)
	}
	root, err := os.ReadFile(filepath.Join(directory, "root.crt"))
	if err != nil {
		t.Fatal(err)
	}
	intermediate, err := os.ReadFile(filepath.Join(directory, "intermediate.crt"))
	if err != nil {
		t.Fatal(err)
	}
	hosts := []string{"allowed.example", "outside.example"}
	effective, skipped, err := effectiveHosts(root, intermediate, hosts, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(effective) != 1 || effective[0] != "allowed.example" || len(skipped) != 1 || skipped[0] != "outside.example" {
		t.Fatalf("effective=%v skipped=%v", effective, skipped)
	}
	effective, skipped, err = effectiveHosts(root, intermediate, hosts, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(effective) != 2 || len(skipped) != 0 {
		t.Fatalf("effective=%v skipped=%v", effective, skipped)
	}
	if _, _, err := effectiveHosts(root, intermediate, []string{"outside.example"}, false); err == nil {
		t.Fatal("accepted no permitted hosts")
	}
}
