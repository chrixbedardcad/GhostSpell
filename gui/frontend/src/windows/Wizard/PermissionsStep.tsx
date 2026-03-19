import { useState, useEffect, useRef } from "react";
import { goCall } from "@/bridge";

/**
 * Permissions step — macOS only.
 * Zen: clear guidance, calm polling, no pressure.
 */
export function PermissionsStep({ onNext }: { onNext: () => void }) {
  const [accessibility, setAccessibility] = useState(false);
  const [inputMonitoring, setInputMonitoring] = useState(false);
  const intervalRef = useRef<number | null>(null);

  useEffect(() => {
    function poll() {
      goCall("checkPermissions").then((raw) => {
        if (!raw) return;
        try {
          const p = JSON.parse(raw);
          setAccessibility(p.accessibility);
          setInputMonitoring(p.postEvent || p.inputMonitoring);
          if (p.accessibility) {
            // Auto-advance when accessibility is granted
            if (intervalRef.current) clearInterval(intervalRef.current);
          }
        } catch { /* ignore */ }
      });
    }

    poll();
    intervalRef.current = window.setInterval(poll, 2000);
    return () => { if (intervalRef.current) clearInterval(intervalRef.current); };
  }, []);

  return (
    <div className="max-w-md mx-auto py-6 space-y-6">
      <div className="text-center mb-6">
        <h2 className="text-lg font-semibold text-text">macOS Permissions</h2>
        <p className="text-xs text-overlay-0 mt-1">
          GhostSpell needs accessibility access to read and type text.
        </p>
      </div>

      {/* Accessibility */}
      <PermissionRow
        name="Accessibility"
        description="Required — lets GhostSpell read selected text and paste results"
        granted={accessibility}
        onGrant={() => goCall("openAccessibilityPane")}
        required
      />

      {/* Input Monitoring */}
      <PermissionRow
        name="Input Monitoring"
        description="For global hotkeys — may require restart after granting"
        granted={inputMonitoring}
        onGrant={() => goCall("openAccessibilityPane")}
      />

      <div className="text-center pt-4">
        <button
          onClick={onNext}
          disabled={!accessibility}
          className={`px-8 py-2.5 rounded-xl text-sm font-medium transition-colors
            ${accessibility
              ? "bg-accent-blue text-crust hover:bg-accent-blue/90"
              : "bg-surface-1 text-overlay-0 cursor-not-allowed"
            }`}
        >
          {accessibility ? "Continue" : "Waiting for Accessibility..."}
        </button>
      </div>
    </div>
  );
}

function PermissionRow({
  name,
  description,
  granted,
  onGrant,
  required,
}: {
  name: string;
  description: string;
  granted: boolean;
  onGrant: () => void;
  required?: boolean;
}) {
  return (
    <div className="bg-surface-0/30 rounded-xl p-4 flex items-center gap-4">
      <div className={`w-8 h-8 rounded-full flex items-center justify-center shrink-0 ${
        granted ? "bg-accent-green/15 text-accent-green" : "bg-accent-yellow/15 text-accent-yellow"
      }`}>
        {granted ? "✓" : "?"}
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm text-text">
          {name}
          {required && <span className="text-accent-red text-[10px] ml-1">required</span>}
        </p>
        <p className="text-xs text-overlay-0 mt-0.5">{description}</p>
      </div>
      {!granted && (
        <button
          onClick={onGrant}
          className="px-3 py-1.5 rounded-lg text-xs font-medium shrink-0
                     bg-accent-blue/15 text-accent-blue hover:bg-accent-blue/25 transition-colors"
        >
          Open Settings
        </button>
      )}
    </div>
  );
}
