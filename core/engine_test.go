package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockTranscriber implements stt.Transcriber for testing.
type mockTranscriber struct {
	text string
	err  error
}

func (m *mockTranscriber) Transcribe(_ context.Context, _ []byte, _ string) (string, error) {
	return m.text, m.err
}

func (m *mockTranscriber) Name() string { return "mock-stt" }

func TestEngine_Transcribe_NoSTT(t *testing.T) {
	e := NewEngine(nil, nil, nil, nil)
	_, err := e.Transcribe(context.Background(), []byte("wav"), "")
	if !errors.Is(err, ErrNoSTT) {
		t.Fatalf("expected ErrNoSTT, got %v", err)
	}
}

func TestEngine_Transcribe_EmptyInput(t *testing.T) {
	e := NewEngine(nil, nil, &mockTranscriber{text: "hello"}, nil)
	_, err := e.Transcribe(context.Background(), nil, "")
	if !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}
}

func TestEngine_Transcribe_Success(t *testing.T) {
	e := NewEngine(nil, nil, &mockTranscriber{text: "  hello world  "}, nil)
	text, err := e.Transcribe(context.Background(), []byte("wav-data"), "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "hello world" {
		t.Fatalf("expected trimmed text, got %q", text)
	}
}

func TestEngine_Process_NoRouter(t *testing.T) {
	e := NewEngine(nil, nil, nil, nil)
	_, err := e.Process(context.Background(), 0, "text")
	if !errors.Is(err, ErrNoRouter) {
		t.Fatalf("expected ErrNoRouter, got %v", err)
	}
}

func TestEngine_HasSTT(t *testing.T) {
	e := NewEngine(nil, nil, nil, nil)
	if e.HasSTT() {
		t.Fatal("expected HasSTT=false with nil transcriber")
	}

	e.SetSTT(&mockTranscriber{})
	if !e.HasSTT() {
		t.Fatal("expected HasSTT=true after SetSTT")
	}

	if e.STTName() != "mock-stt" {
		t.Fatalf("expected STTName=mock-stt, got %q", e.STTName())
	}
}

func TestEngine_SettersAreSafe(t *testing.T) {
	e := NewEngine(nil, nil, nil, nil)

	// Run setters and getters concurrently to detect races.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			e.SetSTT(&mockTranscriber{text: "test"})
			e.SetSTT(nil)
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = e.HasSTT()
		_ = e.STTName()
	}
	<-done
}

func TestEngine_TimeoutForSkill_NoRouter(t *testing.T) {
	e := NewEngine(nil, nil, nil, nil)
	d := e.TimeoutForSkill(0)
	if d != 30*time.Second {
		t.Fatalf("expected 30s default, got %v", d)
	}
}
