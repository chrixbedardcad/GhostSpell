import { useState, useEffect } from "react";
import { goCall } from "@/bridge";

const LANGUAGES = [
  { value: "", label: "Auto (original language)" },
  { value: "English", label: "English" },
  { value: "French", label: "French" },
  { value: "Spanish", label: "Spanish" },
  { value: "German", label: "German" },
  { value: "Italian", label: "Italian" },
  { value: "Portuguese", label: "Portuguese" },
  { value: "Dutch", label: "Dutch" },
  { value: "Russian", label: "Russian" },
  { value: "Chinese", label: "Chinese" },
  { value: "Japanese", label: "Japanese" },
  { value: "Korean", label: "Korean" },
  { value: "Arabic", label: "Arabic" },
];

const VOICE_LANGUAGES = [
  { value: "", label: "Auto-detect" },
  { value: "en", label: "English" },
  { value: "fr", label: "French" },
  { value: "es", label: "Spanish" },
  { value: "de", label: "German" },
  { value: "it", label: "Italian" },
  { value: "pt", label: "Portuguese" },
  { value: "nl", label: "Dutch" },
  { value: "ru", label: "Russian" },
  { value: "zh", label: "Chinese" },
  { value: "ja", label: "Japanese" },
  { value: "ko", label: "Korean" },
  { value: "ar", label: "Arabic" },
];

/**
 * Language tab — unified language configuration.
 * Writing language, voice language, and native language in one place.
 */
export function LanguageTab() {
  const [writingLang, setWritingLang] = useState("");
  const [voiceLang, setVoiceLang] = useState("");
  const [nativeLang, setNativeLang] = useState("");

  useEffect(() => {
    goCall("getConfig").then((raw) => {
      if (!raw) return;
      try {
        const cfg = JSON.parse(raw);
        setWritingLang(cfg.language || "");
        setVoiceLang(cfg.voice?.language || "");
        setNativeLang(cfg.voice?.native_language || "");
      } catch { /* ignore */ }
    });
  }, []);

  function saveWritingLang(value: string) {
    setWritingLang(value);
    goCall("setLanguage", value);
  }

  function saveVoiceLang(value: string) {
    setVoiceLang(value);
    goCall("setVoiceLanguage", value);
  }

  function saveNativeLang(value: string) {
    setNativeLang(value);
    goCall("setVoiceNativeLanguage", value);
  }

  return (
    <div className="space-y-8">
      {/* Writing Language */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Writing Language
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5 space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-text">Text language</p>
              <p className="text-xs text-overlay-0 mt-0.5">
                Skills like Correct, Polish, Ask, and Define will keep text in this language.
                Referenced as {"{{language}}"} in skill prompts.
              </p>
            </div>
            <select
              value={writingLang}
              onChange={(e) => saveWritingLang(e.target.value)}
              className="bg-crust border border-surface-0 rounded-lg px-3 py-1.5 text-xs text-text min-w-[160px]"
            >
              {LANGUAGES.map((l) => (
                <option key={l.value} value={l.value}>{l.label}</option>
              ))}
            </select>
          </div>
        </div>
      </section>

      {/* Voice Language */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Voice Language
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5 space-y-5">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-text">Speaking language</p>
              <p className="text-xs text-overlay-0 mt-0.5">
                The language you speak in. Helps whisper transcribe more accurately.
              </p>
            </div>
            <select
              value={voiceLang}
              onChange={(e) => saveVoiceLang(e.target.value)}
              className="bg-crust border border-surface-0 rounded-lg px-3 py-1.5 text-xs text-text min-w-[160px]"
            >
              {VOICE_LANGUAGES.map((l) => (
                <option key={l.value} value={l.value}>{l.label}</option>
              ))}
            </select>
          </div>

          <div className="h-px bg-surface-0/50" />

          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-text">Native language</p>
              <p className="text-xs text-overlay-0 mt-0.5">
                Your native language. If different from speaking language, the LLM accounts
                for accent-related transcription errors in voice skills.
              </p>
            </div>
            <select
              value={nativeLang}
              onChange={(e) => saveNativeLang(e.target.value)}
              className="bg-crust border border-surface-0 rounded-lg px-3 py-1.5 text-xs text-text min-w-[160px]"
            >
              {VOICE_LANGUAGES.map((l) => (
                <option key={l.value} value={l.value}>{l.label}</option>
              ))}
            </select>
          </div>
        </div>
      </section>
    </div>
  );
}
