import { usePlatform } from "@/hooks/usePlatform";

/**
 * Help tab -- how to use GhostSpell.
 * Zen: clean numbered steps, calm typography, lots of breathing room.
 */
export function HelpTab() {
  const platform = usePlatform();
  const hotkey = platform === "darwin" ? "⌘G" : "Ctrl+G";
  const cycleKey = platform === "darwin" ? "⌘⇧G" : "Ctrl+Shift+G";
  const cancelKey = platform === "darwin" ? "⌘G ⌘G" : "Ctrl+G Ctrl+G";
  const undo = platform === "darwin" ? "⌘Z" : "Ctrl+Z";

  const steps = [
    { num: 1, text: "Select text in any application, or place your cursor in a text field." },
    { num: 2, text: `Press ${hotkey} to activate GhostSpell.` },
    { num: 3, text: "GhostSpell captures the text, sends it to your AI model, and replaces it with the result." },
    { num: 4, text: `Press ${undo} to undo and restore the original text.` },
  ];

  const shortcuts = [
    { action: "Action hotkey",   key: hotkey,    desc: "Activate GhostSpell on selected text" },
    { action: "Cycle skill",     key: cycleKey,  desc: "Switch to the next skill/prompt" },
    { action: "Cancel request",  key: cancelKey, desc: "Press hotkey twice quickly to cancel" },
    { action: "Undo",            key: undo,      desc: "Revert to original text after replace" },
  ];

  const indicators = [
    { status: "Processing", color: "bg-accent-blue",   desc: "AI is processing your request" },
    { status: "Success",    color: "bg-accent-green",  desc: "Text replaced successfully" },
    { status: "Error",      color: "bg-accent-red",    desc: "Something went wrong" },
    { status: "Idle",       color: "bg-overlay-0",     desc: "Ready and waiting" },
    { status: "Voice",      color: "bg-accent-purple",  desc: "Listening for voice input" },
  ];

  const defaultSkills = [
    { icon: "\u270F\uFE0F", name: "Correct",    desc: "Fix spelling and grammar" },
    { icon: "\u2728",       name: "Polish",     desc: "Improve clarity and flow" },
    { icon: "\uD83C\uDF10", name: "Translate",  desc: "Translate to target language" },
    { icon: "\uD83D\uDE02", name: "Funny",      desc: "Rewrite with humor" },
    { icon: "\u2753",       name: "Ask",        desc: "Answer a question about the text" },
    { icon: "\uD83D\uDCD6", name: "Define",     desc: "Define a word or phrase" },
    { icon: "\uD83D\uDCCB", name: "Summarize",  desc: "Condense text into key points" },
  ];

  const tips = [
    { icon: "\u26A1", text: `Press ${hotkey} twice to cancel an active request.` },
    { icon: "\uD83D\uDC7B", text: "Click the ghost indicator to cycle between prompts." },
    { icon: "\uD83D\uDDB1\uFE0F", text: "Right-click the ghost indicator for a prompt menu." },
    { icon: "\u2699\uFE0F", text: "Double-click the ghost indicator to open settings." },
    { icon: "\uD83D\uDCF8", text: "Vision prompts capture a screenshot instead of text." },
    { icon: "\uD83E\uDDE0", text: "Each prompt can use a different AI model -- set it in the Skills tab." },
    { icon: "\uD83C\uDFA4", text: "Voice mode records from your microphone and transcribes with Whisper." },
    { icon: "\uD83D\uDD12", text: "Local models run entirely on your machine -- no data leaves your PC." },
    { icon: "\uD83D\uDD27", text: "You can create custom skills with your own system prompts." },
  ];

  return (
    <div className="space-y-8">
      {/* How It Works */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          How It Works
        </h2>
        <div className="space-y-3">
          {steps.map((s) => (
            <div key={s.num} className="flex gap-4 items-start">
              <span className="shrink-0 w-7 h-7 rounded-full bg-accent-blue/15 text-accent-blue
                             text-xs font-semibold flex items-center justify-center mt-0.5">
                {s.num}
              </span>
              <p className="text-[13px] text-subtext-0 leading-relaxed pt-1">{s.text}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Keyboard Shortcuts */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Keyboard Shortcuts
        </h2>
        <div className="bg-surface-0/30 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-surface-0/50">
                <th className="text-left px-4 py-2.5 text-[11px] font-medium text-overlay-0 uppercase tracking-wider">Action</th>
                <th className="text-left px-4 py-2.5 text-[11px] font-medium text-overlay-0 uppercase tracking-wider">Shortcut</th>
                <th className="text-left px-4 py-2.5 text-[11px] font-medium text-overlay-0 uppercase tracking-wider">Description</th>
              </tr>
            </thead>
            <tbody>
              {shortcuts.map((s) => (
                <tr key={s.action} className="border-b border-surface-0/30 last:border-0">
                  <td className="px-4 py-2.5 text-text font-medium">{s.action}</td>
                  <td className="px-4 py-2.5">
                    <span className="px-2 py-0.5 rounded bg-crust border border-surface-0 text-accent-blue font-mono text-xs">
                      {s.key}
                    </span>
                  </td>
                  <td className="px-4 py-2.5 text-overlay-1">{s.desc}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* Status Indicators */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Status Indicators
        </h2>
        <div className="bg-surface-0/30 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-surface-0/50">
                <th className="text-left px-4 py-2.5 text-[11px] font-medium text-overlay-0 uppercase tracking-wider">Status</th>
                <th className="text-left px-4 py-2.5 text-[11px] font-medium text-overlay-0 uppercase tracking-wider">Indicator</th>
                <th className="text-left px-4 py-2.5 text-[11px] font-medium text-overlay-0 uppercase tracking-wider">Description</th>
              </tr>
            </thead>
            <tbody>
              {indicators.map((ind) => (
                <tr key={ind.status} className="border-b border-surface-0/30 last:border-0">
                  <td className="px-4 py-2.5 text-text font-medium">{ind.status}</td>
                  <td className="px-4 py-2.5">
                    <span className={`inline-block w-3 h-3 rounded-full ${ind.color}`} />
                  </td>
                  <td className="px-4 py-2.5 text-overlay-1">{ind.desc}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* Default Skills */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Default Skills
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5 space-y-2">
          {defaultSkills.map((skill) => (
            <div key={skill.name} className="flex items-baseline gap-3">
              <span className="shrink-0 text-base">{skill.icon}</span>
              <span className="text-sm font-medium text-accent-blue shrink-0 w-20">{skill.name}</span>
              <span className="text-[13px] text-overlay-1">{skill.desc}</span>
            </div>
          ))}
        </div>
      </section>

      {/* Tips */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Tips
        </h2>
        <ul className="space-y-2">
          {tips.map((tip, i) => (
            <li key={i} className="flex gap-3 items-start">
              <span className="shrink-0 text-base mt-0.5">{tip.icon}</span>
              <p className="text-[13px] text-overlay-1 leading-relaxed">{tip.text}</p>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
