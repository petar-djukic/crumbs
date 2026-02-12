package types

import (
	"errors"
	"testing"
)

func TestStashSetValue(t *testing.T) {
	t.Run("context stash", func(t *testing.T) {
		s := &Stash{StashType: StashTypeContext, Version: 1}
		err := s.SetValue(map[string]any{"key": "val"})
		if err != nil {
			t.Fatal(err)
		}
		if s.Version != 2 {
			t.Fatalf("expected version 2, got %d", s.Version)
		}
		if s.LastOperation != StashOpSet {
			t.Fatalf("expected operation %s, got %s", StashOpSet, s.LastOperation)
		}
	})

	t.Run("lock stash rejects SetValue", func(t *testing.T) {
		s := &Stash{StashType: StashTypeLock, Version: 1}
		err := s.SetValue("x")
		if !errors.Is(err, ErrInvalidStashType) {
			t.Fatalf("expected ErrInvalidStashType, got %v", err)
		}
	})
}

func TestStashIncrement(t *testing.T) {
	t.Run("increment counter", func(t *testing.T) {
		s := &Stash{StashType: StashTypeCounter, Version: 1, Value: map[string]any{"value": int64(5)}}
		result, err := s.Increment(3)
		if err != nil {
			t.Fatal(err)
		}
		if result != 8 {
			t.Fatalf("expected 8, got %d", result)
		}
		if s.Version != 2 {
			t.Fatalf("expected version 2, got %d", s.Version)
		}
	})

	t.Run("increment nil value starts at zero", func(t *testing.T) {
		s := &Stash{StashType: StashTypeCounter, Version: 1}
		result, err := s.Increment(1)
		if err != nil {
			t.Fatal(err)
		}
		if result != 1 {
			t.Fatalf("expected 1, got %d", result)
		}
	})

	t.Run("decrement", func(t *testing.T) {
		s := &Stash{StashType: StashTypeCounter, Version: 1, Value: map[string]any{"value": int64(10)}}
		result, err := s.Increment(-3)
		if err != nil {
			t.Fatal(err)
		}
		if result != 7 {
			t.Fatalf("expected 7, got %d", result)
		}
	})

	t.Run("non-counter rejects increment", func(t *testing.T) {
		s := &Stash{StashType: StashTypeResource, Version: 1}
		_, err := s.Increment(1)
		if !errors.Is(err, ErrInvalidStashType) {
			t.Fatalf("expected ErrInvalidStashType, got %v", err)
		}
	})
}

func TestStashLockOperations(t *testing.T) {
	t.Run("acquire and release", func(t *testing.T) {
		s := &Stash{StashType: StashTypeLock, Version: 1}
		if err := s.Acquire("worker-1"); err != nil {
			t.Fatal(err)
		}
		if s.Version != 2 {
			t.Fatalf("expected version 2, got %d", s.Version)
		}
		if err := s.Release("worker-1"); err != nil {
			t.Fatal(err)
		}
		if s.Value != nil {
			t.Fatal("expected nil value after release")
		}
		if s.Version != 3 {
			t.Fatalf("expected version 3, got %d", s.Version)
		}
	})

	t.Run("reentrant acquire", func(t *testing.T) {
		s := &Stash{StashType: StashTypeLock, Version: 1}
		_ = s.Acquire("worker-1")
		v := s.Version
		err := s.Acquire("worker-1")
		if err != nil {
			t.Fatalf("reentrant acquire should succeed, got %v", err)
		}
		if s.Version != v {
			t.Fatal("version should not change on reentrant acquire")
		}
	})

	t.Run("acquire held by another", func(t *testing.T) {
		s := &Stash{StashType: StashTypeLock, Version: 1}
		_ = s.Acquire("worker-1")
		err := s.Acquire("worker-2")
		if !errors.Is(err, ErrLockHeld) {
			t.Fatalf("expected ErrLockHeld, got %v", err)
		}
	})

	t.Run("release by non-holder", func(t *testing.T) {
		s := &Stash{StashType: StashTypeLock, Version: 1}
		_ = s.Acquire("worker-1")
		err := s.Release("worker-2")
		if !errors.Is(err, ErrNotLockHolder) {
			t.Fatalf("expected ErrNotLockHolder, got %v", err)
		}
	})

	t.Run("acquire with empty holder", func(t *testing.T) {
		s := &Stash{StashType: StashTypeLock, Version: 1}
		err := s.Acquire("")
		if !errors.Is(err, ErrInvalidHolder) {
			t.Fatalf("expected ErrInvalidHolder, got %v", err)
		}
	})

	t.Run("non-lock rejects acquire", func(t *testing.T) {
		s := &Stash{StashType: StashTypeCounter, Version: 1}
		err := s.Acquire("worker-1")
		if !errors.Is(err, ErrInvalidStashType) {
			t.Fatalf("expected ErrInvalidStashType, got %v", err)
		}
	})
}
