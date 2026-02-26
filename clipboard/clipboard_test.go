package clipboard

import (
	"fmt"
	"testing"
)

func TestClipboard_ReadWrite(t *testing.T) {
	var store string
	cb := New(
		func() (string, error) { return store, nil },
		func(text string) error { store = text; return nil },
	)

	if err := cb.Write("hello"); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	got, err := cb.Read()
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if got != "hello" {
		t.Errorf("expected 'hello', got '%s'", got)
	}
}

func TestClipboard_SaveRestore(t *testing.T) {
	var store string = "original content"
	cb := New(
		func() (string, error) { return store, nil },
		func(text string) error { store = text; return nil },
	)

	// Save original
	if err := cb.Save(); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	// Overwrite clipboard
	if err := cb.Write("new content"); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	got, _ := cb.Read()
	if got != "new content" {
		t.Errorf("expected 'new content', got '%s'", got)
	}

	// Restore original
	if err := cb.Restore(); err != nil {
		t.Fatalf("unexpected restore error: %v", err)
	}

	got, _ = cb.Read()
	if got != "original content" {
		t.Errorf("expected 'original content', got '%s'", got)
	}
}

func TestClipboard_RestoreWithoutSave(t *testing.T) {
	cb := New(
		func() (string, error) { return "", nil },
		func(text string) error { return nil },
	)

	// Should be a no-op, not an error
	if err := cb.Restore(); err != nil {
		t.Fatalf("unexpected error restoring without save: %v", err)
	}
}

func TestClipboard_NilFunctions(t *testing.T) {
	cb := New(nil, nil)

	_, err := cb.Read()
	if err == nil {
		t.Fatal("expected error for nil read function")
	}

	err = cb.Write("test")
	if err == nil {
		t.Fatal("expected error for nil write function")
	}

	err = cb.Save()
	if err == nil {
		t.Fatal("expected error for nil read function on save")
	}
}

func TestClipboard_ReadError(t *testing.T) {
	cb := New(
		func() (string, error) { return "", fmt.Errorf("read failed") },
		func(text string) error { return nil },
	)

	_, err := cb.Read()
	if err == nil {
		t.Fatal("expected read error")
	}
}

func TestClipboard_WriteError(t *testing.T) {
	cb := New(
		func() (string, error) { return "", nil },
		func(text string) error { return fmt.Errorf("write failed") },
	)

	err := cb.Write("test")
	if err == nil {
		t.Fatal("expected write error")
	}
}

func TestClipboard_Clear(t *testing.T) {
	var store string
	cb := New(
		func() (string, error) { return store, nil },
		func(text string) error { store = text; return nil },
	)

	// Write something, then clear, verify read returns ""
	if err := cb.Write("hello"); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if err := cb.Clear(); err != nil {
		t.Fatalf("unexpected clear error: %v", err)
	}
	got, err := cb.Read()
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string after clear, got %q", got)
	}
}

func TestClipboard_ClearFallback(t *testing.T) {
	var store string = "content"
	cb := New(
		func() (string, error) { return store, nil },
		func(text string) error { store = text; return nil },
	)
	// No clearFn set — should fall back to writeFn("")
	if err := cb.Clear(); err != nil {
		t.Fatalf("unexpected clear error: %v", err)
	}
	if store != "" {
		t.Errorf("expected store to be empty after fallback clear, got %q", store)
	}
}

func TestClipboard_ClearWithCustomFn(t *testing.T) {
	called := false
	cb := New(
		func() (string, error) { return "", nil },
		func(text string) error { return nil },
	).WithClear(func() error {
		called = true
		return nil
	})

	if err := cb.Clear(); err != nil {
		t.Fatalf("unexpected clear error: %v", err)
	}
	if !called {
		t.Error("expected custom clearFn to be called")
	}
}

func TestClipboard_ClearNilFunctions(t *testing.T) {
	cb := New(nil, nil)
	err := cb.Clear()
	if err == nil {
		t.Fatal("expected error when both clearFn and writeFn are nil")
	}
}
