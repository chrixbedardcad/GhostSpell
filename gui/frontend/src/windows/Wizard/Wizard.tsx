import { useState, useEffect } from "react";
import { goCall } from "@/bridge";
import { usePlatform } from "@/hooks/usePlatform";
import { WelcomeStep } from "./WelcomeStep";
import { PermissionsStep } from "./PermissionsStep";
import { ModelStep } from "./ModelStep";
import { GPUStep } from "./GPUStep";
import { VoiceStep } from "./VoiceStep";
import { ReadyStep } from "./ReadyStep";

type Step = "welcome" | "permissions" | "model" | "gpu" | "voice" | "ready";

/**
 * Setup Wizard — first-launch flow.
 * Every step has a "Skip" option. Closing the wizard = app runs as empty shell.
 * The wizard is a guide, not a gate.
 */
export function WizardWindow() {
  const platform = usePlatform();
  const [step, setStep] = useState<Step>("welcome");
  const [returnToSettings, setReturnToSettings] = useState(false);
  const [chosenLocal, setChosenLocal] = useState(false);

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

  // Steps array for dot navigation (filtered by platform).
  const allSteps: Step[] = platform === "darwin"
    ? ["welcome", "permissions", "model", "gpu", "voice", "ready"]
    : ["welcome", "model", "gpu", "voice", "ready"];
  const stepIndex = allSteps.indexOf(step);
  const totalSteps = allSteps.length;

  function next() {
    switch (step) {
      case "welcome":
        setStep(platform === "darwin" ? "permissions" : "model");
        break;
      case "permissions":
        setStep("model");
        break;
      case "model":
        // If user chose local model, offer GPU step. Otherwise skip to voice.
        setStep("gpu");
        break;
      case "gpu":
        setStep("voice");
        break;
      case "voice":
        setStep("ready");
        break;
    }
  }

  async function skipSetup() {
    await goCall("wizardSkip");
    goCall("closeWindow");
  }

  return (
    <div className="h-full flex flex-col bg-base">
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-4 border-b border-surface-0/40 shrink-0">
        <div className="flex items-center gap-3">
          <img src="/ghost-icon.png" alt="" className="w-7 h-7 opacity-80" />
          <span className="text-sm font-medium text-subtext-1">GhostSpell Setup</span>
        </div>
        {returnToSettings ? (
          <button
            onClick={() => goCall("closeWindow")}
            className="text-xs text-overlay-0 hover:text-subtext-0 transition-colors"
          >
            Cancel
          </button>
        ) : step !== "ready" ? (
          <button
            onClick={skipSetup}
            className="text-xs text-overlay-0 hover:text-subtext-0 transition-colors"
          >
            Skip Setup
          </button>
        ) : null}
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
        {step === "welcome" && <WelcomeStep onNext={next} onSkip={skipSetup} />}
        {step === "permissions" && <PermissionsStep onNext={next} />}
        {step === "model" && (
          <ModelStep
            onNext={(isLocal) => { setChosenLocal(isLocal); next(); }}
            onSkip={() => { setChosenLocal(false); next(); }}
            returnToSettings={returnToSettings}
          />
        )}
        {step === "gpu" && <GPUStep onNext={next} hasLocalModel={chosenLocal} />}
        {step === "voice" && <VoiceStep onNext={next} />}
        {step === "ready" && <ReadyStep />}
      </div>
    </div>
  );
}
