import { goCall } from "@/bridge";
import { usePlatform } from "@/hooks/usePlatform";

/**
 * Ready step — setup complete, show summary.
 * Zen: celebratory but calm. Clear next action.
 */
export function ReadyStep() {
  const platform = usePlatform();
  const hotkey = platform === "darwin" ? "⌘G" : "Ctrl+G";

  async function finish() {
    await goCall("wizardComplete");
    goCall("closeWindow");
  }

  return (
    <div className="flex flex-col items-center justify-center text-center max-w-md mx-auto py-8">
      <div className="text-5xl mb-6 opacity-80">👻</div>
      <h2 className="text-xl font-semibold text-text tracking-tight mb-2">
        You're All Set
      </h2>
      <p className="text-sm text-overlay-1 leading-relaxed mb-8">
        GhostSpell is ready. Select any text and press{" "}
        <span className="inline-flex items-center px-2 py-0.5 rounded-md bg-crust border border-surface-0
                        text-xs font-mono text-accent-blue mx-1">
          {hotkey}
        </span>{" "}
        to activate.
      </p>

      <div className="w-full space-y-3 text-left mb-8">
        <Tip icon="💡" text="Press the hotkey twice to cancel an active request" />
        <Tip icon="👻" text="Click the ghost indicator to switch skills" />
        <Tip icon="⚙️" text="Right-click the tray icon to access settings anytime" />
        <Tip icon="🎤" text="Voice, GPU, and extra models can be added in Settings" />
      </div>

      <button
        onClick={finish}
        className="px-8 py-2.5 rounded-xl text-sm font-medium
                   bg-accent-blue text-crust hover:bg-accent-blue/90
                   transition-colors"
      >
        Start Using GhostSpell
      </button>
    </div>
  );
}

function Tip({ icon, text }: { icon: string; text: string }) {
  return (
    <div className="flex items-center gap-3 bg-surface-0/20 rounded-lg p-3">
      <span className="text-lg shrink-0">{icon}</span>
      <p className="text-xs text-overlay-1">{text}</p>
    </div>
  );
}
