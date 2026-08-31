package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

var pickerItems = []string{"alpha — /repos/alpha", "beta — /repos/beta", "gamma — /repos/gamma"}

func TestChooseFromListValidChoice(t *testing.T) {
	var out bytes.Buffer
	idx, err := chooseFromList(&out, strings.NewReader("2\n"), "Built graphs:", pickerItems)
	if err != nil {
		t.Fatalf("chooseFromList() error = %v", err)
	}
	if idx != 1 {
		t.Errorf("expected 0-based index 1 for input '2', got %d", idx)
	}
	rendered := out.String()
	for i, want := range []string{"1) alpha", "2) beta", "3) gamma"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("numbered list missing %q:\n%s", want, rendered)
		}
		_ = i
	}
}

func TestChooseFromListTrimsWhitespace(t *testing.T) {
	idx, err := chooseFromList(io.Discard, strings.NewReader("  3 \n"), "", pickerItems)
	if err != nil || idx != 2 {
		t.Errorf("expected index 2 after trimming, got %d, err=%v", idx, err)
	}
}

func TestChooseFromListOutOfRange(t *testing.T) {
	for _, input := range []string{"0\n", "4\n", "-1\n"} {
		if _, err := chooseFromList(io.Discard, strings.NewReader(input), "", pickerItems); err == nil {
			t.Errorf("expected an error for out-of-range input %q", input)
		}
	}
}

func TestChooseFromListNonNumeric(t *testing.T) {
	if _, err := chooseFromList(io.Discard, strings.NewReader("banana\n"), "", pickerItems); err == nil {
		t.Error("expected an error for non-numeric input")
	}
}

func TestChooseFromListEOF(t *testing.T) {
	if _, err := chooseFromList(io.Discard, strings.NewReader(""), "", pickerItems); err == nil {
		t.Error("expected an error on EOF (no input)")
	}
}

func TestChooseFromListEmptyItems(t *testing.T) {
	if _, err := chooseFromList(io.Discard, strings.NewReader("1\n"), "", nil); err == nil {
		t.Error("expected an error when there are no items")
	}
}

func TestConfirmPromptAcceptsYesVariants(t *testing.T) {
	for _, input := range []string{"y\n", "Y\n", "yes\n", "YES\n"} {
		ok, err := confirmPrompt(io.Discard, strings.NewReader(input), "?")
		if err != nil || !ok {
			t.Errorf("input %q: expected confirmed, got ok=%v err=%v", input, ok, err)
		}
	}
}

func TestConfirmPromptDefaultsToNo(t *testing.T) {
	for _, input := range []string{"n\n", "no\n", "\n", "anything else\n"} {
		ok, err := confirmPrompt(io.Discard, strings.NewReader(input), "?")
		if err != nil || ok {
			t.Errorf("input %q: expected not confirmed, got ok=%v err=%v", input, ok, err)
		}
	}
}

func TestConfirmPromptEOFIsAnError(t *testing.T) {
	if _, err := confirmPrompt(io.Discard, strings.NewReader(""), "?"); err == nil {
		t.Error("expected an error on EOF so callers can tell 'no user' from 'user said no'")
	}
}
