/**
 * Welcome step — introduces GhostSpell.
 * Zen: centered, calm, inviting.
 */
export function WelcomeStep({ onNext }: { onNext: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center text-center max-w-md mx-auto py-8">
      <img src="/ghost-icon.png" alt="" className="w-20 h-20 mb-6 opacity-80" />
      <h2 className="text-2xl font-semibold text-text tracking-tight mb-2">
        Welcome to GhostSpell
      </h2>
      <p className="text-sm text-overlay-1 leading-relaxed mb-8">
        AI-powered text correction and rewriting. Select text anywhere,
        press a hotkey, and let AI improve it instantly.
      </p>

      <div className="space-y-4 text-left w-full mb-8">
        <Step num={1} text="Select text in any application" />
        <Step num={2} text="Press Ctrl+G (or ⌘G on Mac) to activate" />
        <Step num={3} text="GhostSpell replaces it with the improved version" />
      </div>

      <button
        onClick={onNext}
        className="px-8 py-2.5 rounded-xl text-sm font-medium
                   bg-accent-blue text-crust hover:bg-accent-blue/90
                   transition-colors"
      >
        Get Started
      </button>
    </div>
  );
}

function Step({ num, text }: { num: number; text: string }) {
  return (
    <div className="flex items-center gap-4">
      <span className="shrink-0 w-8 h-8 rounded-full bg-accent-blue/10 text-accent-blue
                       text-sm font-semibold flex items-center justify-center">
        {num}
      </span>
      <p className="text-sm text-subtext-0">{text}</p>
    </div>
  );
}
