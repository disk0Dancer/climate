package confirm_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/disk0Dancer/climate/internal/confirm"
)

func TestAskYes(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	ok, err := confirm.Ask(strings.NewReader("yes\n"), &out, "Delete it?")
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if !ok {
		t.Fatal("Ask() should return true for yes")
	}
	if !strings.Contains(out.String(), "Delete it? [y/N]:") {
		t.Fatal("prompt should be written")
	}
}

func TestAskNoByDefault(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	ok, err := confirm.Ask(strings.NewReader("\n"), &out, "Delete it?")
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if ok {
		t.Fatal("Ask() should return false on empty input")
	}
}

func TestAskRetriesOnInvalidInput(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	ok, err := confirm.Ask(strings.NewReader("maybe\ny\n"), &out, "Delete it?")
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if !ok {
		t.Fatal("Ask() should return true after retrying with yes")
	}
	if !strings.Contains(out.String(), "Please answer yes or no.") {
		t.Fatal("Ask() should explain invalid input")
	}
}
