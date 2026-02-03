package services

import (
	"math/rand"
	"testing"
)

func TestChoosePositions_ExcludesFreePos(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	free := 12
	got, err := choosePositions(5, &free, 24, true, rng)
	if err != nil {
		t.Fatalf("choosePositions error: %v", err)
	}
	if len(got) != 24 {
		t.Fatalf("expected 24 positions, got %d", len(got))
	}
	for _, p := range got {
		if p == free {
			t.Fatalf("expected free position %d to be excluded", free)
		}
		if p < 0 || p >= 25 {
			t.Fatalf("position out of range: %d", p)
		}
	}
}

func TestChoosePositions_DeterministicWhenNotShuffled(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	free := 4
	got, err := choosePositions(3, &free, 6, false, rng)
	if err != nil {
		t.Fatalf("choosePositions error: %v", err)
	}
	want := []int{0, 1, 2, 3, 5, 6}
	if len(got) != len(want) {
		t.Fatalf("expected %d positions, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected positions at %d: want %d got %d", i, want[i], got[i])
		}
	}
}

func TestChoosePositions_ErrorsWhenOverCapacity(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	free := 0
	_, err := choosePositions(2, &free, 4, false, rng)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestNormalizeTemplateItems_TrimsAndRejectsEmpty(t *testing.T) {
	got, err := normalizeTemplateItems([]string{"  a  ", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected normalized items: %#v", got)
	}

	if _, err := normalizeTemplateItems([]string{"ok", " "}); err == nil {
		t.Fatalf("expected error for empty item")
	}
}

func TestValidateTemplateCapacity(t *testing.T) {
	if err := validateTemplateCapacity(2, true, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := validateTemplateCapacity(2, true, 4); err == nil {
		t.Fatalf("expected error for over-capacity")
	}
}

func TestWithCopySuffix_RespectsTitleLimit(t *testing.T) {
	base := ""
	for i := 0; i < 200; i++ {
		base += "a"
	}
	got := withCopySuffix(base, 1)
	if len(got) > 100 {
		t.Fatalf("expected <= 100 chars, got %d", len(got))
	}
}

