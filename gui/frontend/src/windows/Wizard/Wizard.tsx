import { useState, useEffect } from "react";
import { goCall, openURL } from "@/bridge";
import { usePlatform } from "@/hooks/usePlatform";
import { WelcomeStep } from "./WelcomeStep";
import { PermissionsStep } from "./PermissionsStep";
import { ModelStep } from "./ModelStep";
import { ReadyStep } from "./ReadyStep";

/**
 * Setup Wizard — first-launch flow.
 * Zen: clean steps, calm transitions, focused guidance.
 */
export function WizardWindow() {
  const platform = usePlatform();
  const [step, setStep] = useState<"welcome" | "permissions" | "model" | "ready">("welcome");
  const [returnToSettings, setReturnToSettings] = useState(false);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get("returnTo") === "settings") {
      setReturnToSettings(true);
      setStep("model");
      return;
    }

    // Smart skip: if macOS permissions already granted, skip to model
    if (platform === "darwin") {
      goCall("checkPermissions").then((raw) => {
        if (raw) {
          try {
            const perms = JSON.parse(raw);
            if (perms.accessibility) {
              setStep("model");
            } else {
              setStep("permissions");
            }
          } catch { setStep("welcome"); }
        }
      });
    }
  }, [platform]);

  const totalSteps = platform === "darwin" ? 4 : 3;
  const stepIndex = step === "welcome" ? 0
    : step === "permissions" ? 1
    : step === "model" ? (platform === "darwin" ? 2 : 1)
    : (platform === "darwin" ? 3 : 2);

  function next() {
    if (step === "welcome") {
      setStep(platform === "darwin" ? "permissions" : "model");
    } else if (step === "permissions") {
      setStep("model");
    } else if (step === "model") {
      setStep("ready");
    }
  }

  return (
    <div className="h-full flex flex-col bg-base">
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-4 border-b border-surface-0/40 shrink-0">
        <div className="flex items-center gap-3">
          <img src="/ghost-icon.png" alt="" className="w-7 h-7 opacity-80" />
          <span className="text-sm font-medium text-subtext-1">GhostSpell Setup</span>
        </div>
        {returnToSettings && (
          <button
            onClick={() => goCall("closeWindow")}
            className="text-xs text-overlay-0 hover:text-subtext-0 transition-colors"
          >
            Cancel
          </button>
        )}
      </div>

      {/* Step dots */}
      {!returnToSettings && (
        <div className="flex justify-center gap-2 py-3 shrink-0">
          {Array.from({ length: totalSteps }).map((_, i) => (
            <div
              key={i}
              className={`w-2 h-2 rounded-full transition-colors ${
                i === stepIndex ? "bg-accent-blue" : i < stepIndex ? "bg-accent-blue/40" : "bg-surface-1"
              }`}
            />
          ))}
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-y-auto px-6 py-6">
        {step === "welcome" && <WelcomeStep onNext={next} />}
        {step === "permissions" && <PermissionsStep onNext={next} />}
        {step === "model" && <ModelStep onNext={next} returnToSettings={returnToSettings} />}
        {step === "ready" && <ReadyStep />}
      </div>
    </div>
  );
}
