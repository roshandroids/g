package prompt

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTerminalConfirm(t *testing.T) {
	tests := []struct {
		name   string
		answer string
		want   bool
	}{
		{name: "lower case yes", answer: "y\n", want: true},
		{name: "yes", answer: "yes\n", want: true},
		{name: "upper case yes", answer: "Y\n", want: true},
		{name: "mixed case yes", answer: "YeS\n", want: true},
		{name: "padded yes", answer: "  yes  \n", want: true},
		{name: "no", answer: "n\n", want: false},
		{name: "no spelled out", answer: "no\n", want: false},
		{name: "empty answer", answer: "\n", want: false},
		{name: "anything else", answer: "maybe\n", want: false},
		{name: "yes as a prefix of another word", answer: "yesterday\n", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			terminal := New(strings.NewReader(test.answer), &out)

			got, err := terminal.Confirm(context.Background(), "Proceed?")
			if err != nil {
				t.Fatalf("Confirm() error = %v, want nil", err)
			}
			if got != test.want {
				t.Errorf("Confirm(%q) = %t, want %t", test.answer, got, test.want)
			}
			if !strings.Contains(out.String(), "Proceed?") {
				t.Errorf("Confirm() asked %q, want it to include the question", out.String())
			}
			// The default has to be visible, or the user cannot tell what
			// pressing enter will do.
			if !strings.Contains(out.String(), "[y/N]") {
				t.Errorf("Confirm() asked %q, want it to show the default", out.String())
			}
		})
	}
}

// An input that ends without an answer is a no, never an accidental yes.
func TestTerminalConfirmTreatsEndOfInputAsNo(t *testing.T) {
	var out bytes.Buffer
	terminal := New(strings.NewReader(""), &out)

	got, err := terminal.Confirm(context.Background(), "Proceed?")
	if err != nil {
		t.Fatalf("Confirm() error = %v, want nil", err)
	}
	if got {
		t.Error("Confirm() = true, want false when there is nobody to ask")
	}
}

func TestTerminalConfirmReadsOnlyTheFirstLine(t *testing.T) {
	var out bytes.Buffer
	terminal := New(strings.NewReader("yes\nno\n"), &out)

	first, err := terminal.Confirm(context.Background(), "First?")
	if err != nil {
		t.Fatalf("Confirm() error = %v, want nil", err)
	}
	if !first {
		t.Error("first Confirm() = false, want true")
	}

	// The reader has to be shared between calls, or the second question would
	// consume input the first one already buffered.
	second, err := terminal.Confirm(context.Background(), "Second?")
	if err != nil {
		t.Fatalf("Confirm() error = %v, want nil", err)
	}
	if second {
		t.Error("second Confirm() = true, want false for the second line")
	}
}

func TestTerminalConfirmReportsAWriteFailure(t *testing.T) {
	terminal := New(strings.NewReader("y\n"), failingWriter{})

	if _, err := terminal.Confirm(context.Background(), "Proceed?"); err == nil {
		t.Fatal("Confirm() error = nil, want the write failure")
	}
}

func TestTerminalConfirmReportsAReadFailure(t *testing.T) {
	terminal := New(failingReader{}, &bytes.Buffer{})

	if _, err := terminal.Confirm(context.Background(), "Proceed?"); err == nil {
		t.Fatal("Confirm() error = nil, want the read failure")
	}
}

func TestTerminalConfirmHonoursACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	terminal := New(strings.NewReader("yes\n"), &bytes.Buffer{})

	got, err := terminal.Confirm(ctx, "Proceed?")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Confirm() error = %v, want context.Canceled", err)
	}
	if got {
		t.Error("Confirm() = true, want false for a cancelled context")
	}
}

func TestDenyRefusesEverything(t *testing.T) {
	got, err := Deny{}.Confirm(context.Background(), "Proceed?")
	if err != nil {
		t.Fatalf("Confirm() error = %v, want nil", err)
	}
	if got {
		t.Error("Confirm() = true, want false")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }
