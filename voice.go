package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chrixbedardcad/GhostSpell/clipboard"
	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/gui"
	"github.com/chrixbedardcad/GhostSpell/keyboard"
	"github.com/chrixbedardcad/GhostSpell/mode"
	"github.com/chrixbedardcad/GhostSpell/sound"
	"github.com/chrixbedardcad/GhostSpell/stats"
	"github.com/chrixbedardcad/GhostSpell/stt"
)

// voiceRecording tracks whether we're currently recording.
var voiceRecording atomic.Bool
var voiceStopCh chan struct{}
var voiceStopMu sync.Mutex

// processVoice handles the voice prompt path:
// record audio → transcribe → process with LLM → paste result.
// Called from processMode when the active prompt has Voice: true.
func processVoice(
	promptName string,
	promptIdx int,
	cfg *config.Config,
	router *mode.Router,
	cb *clipboard.Clipboard,
	kb keyboard.Simulator,
	mu *sync.Mutex,
	cancelCtx context.Context,
	startAnim func(),
	stopAnim func(),
	transcriber stt.Transcriber,
) {
	fmt.Printf("[voice] processVoice called: prompt=%s, transcriber=%v\n", promptName, transcriber != nil)
	slog.Info("[voice] processVoice called", "prompt", promptName, "has_transcriber", transcriber != nil)

	// Start recording.
	voiceRecording.Store(true)

	voiceStopMu.Lock()
	voiceStopCh = make(chan struct{})
	stopCh := voiceStopCh
	voiceStopMu.Unlock()

	slog.Info("[voice] Recording started", "prompt", promptName)
	fmt.Printf("[%s] Voice recording started...\n", promptName)
	sound.PlayMicStart()

	// Save the foreground window before showing any indicator.
	// On Windows, the indicator steals focus — without this, paste goes to the wrong app.
	kb.SaveForegroundWindow()

	// Show recording indicator with recording flag for red dot + voice pulse.
	gui.ShowRecordingIndicator()

	// Record audio via malgo (miniaudio).
	recorder := sound.NewRecorder()
	if !recorder.MicAvailable() {
		slog.Error("[voice] No microphone available")
		fmt.Println("[voice] ERROR: No microphone found")
		gui.HideIndicator()
		gui.PopIndicator("🎙️❌", "No microphone")
		sound.PlayError()
		return
	}

	// Poll audio level during recording for visual feedback on the indicator.
	recCtx, recCancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				gui.EmitAudioLevel(recorder.Level())
			case <-recCtx.Done():
				return
			}
		}
	}()

	fmt.Println("[voice] Starting audio capture...")
	wavData, duration, err := recorder.Record(cancelCtx, stopCh)

	// IMMEDIATELY clear the recording flag so Ctrl+G goes to "cancel request."
	voiceRecording.Store(false)
	voiceStopMu.Lock()
	voiceStopCh = nil
	voiceStopMu.Unlock()

	recCancel() // stop level polling

	if err != nil {
		slog.Error("[voice] Recording failed", "error", err)
		gui.HideIndicator()
		gui.PopIndicator("🎙️❌", "Recording failed")
		sound.PlayError()
		return
	}

	// Check if cancelled.
	if cancelCtx.Err() != nil {
		slog.Info("[voice] Cancelled during recording")
		gui.HideIndicator()
		return
	}

	slog.Info("[voice] Recording complete", "duration", duration, "wav_size", len(wavData))
	// PlayMicStop for timeout/max-duration stop. F7 stop already plays it in process.go.

	// Show transcribing indicator IMMEDIATELY — no gap.
	voiceModelName := cfg.Voice.Model
	if transcriber != nil {
		voiceModelName = transcriber.Name() + " (" + cfg.Voice.Model + ")"
	}
	gui.ShowIndicator("🎙️", "Transcribing...", voiceModelName)
	sound.PlayClick()
	sound.StartWorkingLoop()
	transcribeStart := time.Now()

	if transcriber == nil {
		slog.Error("[voice] No STT provider configured")
		gui.HideIndicator()
		gui.PopIndicator("🎙️❌", "No voice model")
		sound.PlayError()
		return
	}

	// Get language preference.
	language := ""
	if cfg.Voice.Language != "" {
		language = cfg.Voice.Language
	}

	transcript, err := transcriber.Transcribe(cancelCtx, wavData, language)
	if err != nil {
		slog.Error("[voice] Transcription failed", "error", err)
		gui.HideIndicator()
		gui.PopIndicator("🎙️❌", "Transcription failed")
		sound.PlayError()
		return
	}

	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		slog.Warn("[voice] Empty transcription")
		gui.HideIndicator()
		gui.PopIndicator("🎙️", "No speech detected")
		sound.PlayError()
		return
	}

	slog.Info("[voice] Transcription complete", "text_len", len(transcript), "text", transcript)
	fmt.Printf("[%s] Transcribed: %s\n", promptName, transcript)

	// Check voice mode — dictation (paste directly) or skill (process with LLM).
	voiceMode := "skill"
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) && cfg.Prompts[promptIdx].VoiceMode != "" {
		voiceMode = cfg.Prompts[promptIdx].VoiceMode
	}

	transcribeElapsed := time.Since(transcribeStart)

	if voiceMode == "dictation" {
		// Direct paste — no LLM processing.
		slog.Info("[voice] Dictation mode — pasting transcript directly")
		kb.RestoreForegroundWindow() // ensure paste goes to target app, not indicator
		if err := cb.Write(transcript); err != nil {
			slog.Error("[voice] Clipboard write failed", "error", err)
			gui.HideIndicator()
			sound.PlayError()
			return
		}
		time.Sleep(50 * time.Millisecond)
		kb.Paste()
		time.Sleep(150 * time.Millisecond)
		gui.HideIndicator()
		sound.PlaySuccess()
		if appStats != nil {
			appStats.Record(stats.Entry{
				Timestamp:   time.Now(),
				Prompt:      promptName,
				PromptIcon:  "🎙️",
				Provider:    "ghost-voice",
				Model:       cfg.Voice.Model,
				ModelLabel:  "Ghost Voice",
				InputChars:  len(wavData),
				OutputChars: len(transcript),
				OutputWords: len(strings.Fields(transcript)),
				DurationMs:  transcribeElapsed.Milliseconds(),
				Status:      "success",
			})
		}
		fmt.Printf("[%s] Dictation complete (%d chars)\n", promptName, len(transcript))
		return
	}

	// Skill mode — process transcript with active prompt.
	if router == nil {
		slog.Error("[voice] No LLM router available — cannot process voice skill")
		gui.HideIndicator()
		gui.PopIndicator("🎙️❌", "No model configured")
		sound.PlayError()
		return
	}

	sound.PlayToggle()
	sound.StartWorkingLoop()

	promptIcon := ""
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) {
		promptIcon = cfg.Prompts[promptIdx].Icon
	}
	modelLabel := cfg.DefaultModel
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) && cfg.Prompts[promptIdx].LLM != "" {
		modelLabel = cfg.Prompts[promptIdx].LLM
	}
	indicatorModel := modelLabel
	if me, ok := cfg.Models[modelLabel]; ok && me.Model != "" {
		indicatorModel = me.Model
	}
	gui.ShowIndicator(promptIcon, promptName, indicatorModel)

	// Add native language context so the LLM can correct accent-related
	// transcription errors (e.g., French speaker saying English words).
	textToSend := transcript
	if cfg.Voice.NativeLanguage != "" {
		textToSend = "[Speaker's native language: " + cfg.Voice.NativeLanguage +
			". The transcription may contain errors due to accent. Correct accordingly.]\n\n" + transcript
	}

	timeout := time.Duration(router.TimeoutForPrompt(promptIdx)) * time.Millisecond
	ctx, cancel := context.WithTimeout(cancelCtx, timeout)
	defer cancel()

	resp, err := router.Process(ctx, promptIdx, textToSend)
	gui.HideIndicator()

	if err != nil {
		slog.Error("[voice] LLM processing failed", "error", err)
		sound.StopWorkingLoop()

		if ctx.Err() == context.Canceled && !strings.Contains(err.Error(), "deadline exceeded") {
			gui.PopIndicator("🛑", "Cancelled")
			return
		}
		gui.PopIndicator("🎙️❌", "Processing failed")
		sound.PlayError()
		return
	}

	result := strings.TrimSpace(resp.Text)
	if result == "" {
		slog.Warn("[voice] LLM returned empty result")
		gui.HideIndicator()
		sound.PlayError()
		return
	}

	// Restore focus to target app before pasting.
	kb.RestoreForegroundWindow()

	// Check display mode.
	displayMode := ""
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) {
		displayMode = cfg.Prompts[promptIdx].DisplayMode
	}

	if displayMode == "popup" {
		gui.ShowResult(result, promptName, promptIcon, modelLabel)
		sound.PlaySuccess()
		return
	}

	if displayMode == "append" {
		// Append: paste result at cursor (no selection to deselect in voice mode).
		if err := cb.Write(result); err != nil {
			slog.Error("[voice] Clipboard write failed (append)", "error", err)
			sound.PlayError()
			return
		}
		time.Sleep(50 * time.Millisecond)
		kb.Paste()
		time.Sleep(150 * time.Millisecond)
		sound.PlaySuccess()
		slog.Info("[voice] Append complete", "prompt", promptName, "result_len", len(result))
		fmt.Printf("[%s] Voice append complete (%d chars)\n", promptName, len(result))
		return
	}

	// Default: paste result.
	if err := cb.Write(result); err != nil {
		slog.Error("[voice] Clipboard write failed", "error", err)
		sound.PlayError()
		return
	}
	time.Sleep(50 * time.Millisecond)
	kb.Paste()
	time.Sleep(150 * time.Millisecond)
	sound.PlaySuccess()

	slog.Info("[voice] Complete", "prompt", promptName, "transcript_len", len(transcript), "result_len", len(result))
	fmt.Printf("[%s] Voice complete (%d chars → %d chars)\n", promptName, len(transcript), len(result))
}
