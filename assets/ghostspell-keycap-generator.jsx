import { useState, useRef, useCallback } from "react";

const PRESETS = [
  "G", "H", "O", "S", "T", "Ctrl", "Alt", "Shift", "Tab", "Esc",
  "Enter", "⌘", "⌫", "Space", "F6", "F7", "F8", "↑", "↓", "←", "→"
];

const STYLES = {
  ghost: {
    label: "Ghost Dark",
    bg: "linear-gradient(145deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%)",
    keyTop: "#1e1e3a",
    keySide: "#12122a",
    keyBorder: "#3a3a6a",
    text: "#c4b5fd",
    glow: "rgba(139, 92, 246, 0.4)",
    shadowColor: "rgba(139, 92, 246, 0.15)",
  },
  midnight: {
    label: "Midnight",
    bg: "linear-gradient(145deg, #0a0a0a 0%, #1a1a1a 100%)",
    keyTop: "#222",
    keySide: "#111",
    keyBorder: "#444",
    text: "#e0e0e0",
    glow: "rgba(255, 255, 255, 0.1)",
    shadowColor: "rgba(0,0,0,0.5)",
  },
  retro: {
    label: "Retro Cream",
    bg: "linear-gradient(145deg, #d4c5a9 0%, #c2b280 100%)",
    keyTop: "#f5f0e1",
    keySide: "#d4c9a8",
    keyBorder: "#b8a88a",
    text: "#3a3a3a",
    glow: "rgba(0,0,0,0.05)",
    shadowColor: "rgba(0,0,0,0.2)",
  },
  neon: {
    label: "Neon",
    bg: "linear-gradient(145deg, #0d0d0d 0%, #1a0a2e 100%)",
    keyTop: "#111",
    keySide: "#0a0a0a",
    keyBorder: "#00ff88",
    text: "#00ff88",
    glow: "rgba(0, 255, 136, 0.5)",
    shadowColor: "rgba(0, 255, 136, 0.2)",
  },
};

function drawKeycap(canvas, label, style, size) {
  const s = STYLES[style];
  const ctx = canvas.getContext("2d");
  const dpr = 2;

  const isWide = ["Ctrl", "Alt", "Shift", "Tab", "Enter", "Space", "⌫"].includes(label);
  const w = isWide ? size * 2.2 : size;
  const h = size;
  const pad = size * 0.15;

  canvas.width = (w + pad * 2) * dpr;
  canvas.height = (h + pad * 2 + size * 0.12) * dpr;
  canvas.style.width = `${w + pad * 2}px`;
  canvas.style.height = `${h + pad * 2 + size * 0.12}px`;
  ctx.scale(dpr, dpr);

  ctx.clearRect(0, 0, canvas.width, canvas.height);

  const x = pad;
  const y = pad;
  const r = size * 0.12;
  const depth = size * 0.08;

  // Shadow
  ctx.shadowColor = s.shadowColor;
  ctx.shadowBlur = size * 0.2;
  ctx.shadowOffsetY = size * 0.05;

  // Key side (3D depth)
  ctx.fillStyle = s.keySide;
  roundRect(ctx, x, y + depth, w, h, r);
  ctx.fill();

  ctx.shadowColor = "transparent";
  ctx.shadowBlur = 0;
  ctx.shadowOffsetY = 0;

  // Key top
  ctx.fillStyle = s.keyTop;
  roundRect(ctx, x, y, w, h - depth, r);
  ctx.fill();

  // Border
  ctx.strokeStyle = s.keyBorder;
  ctx.lineWidth = 1.5;
  roundRect(ctx, x, y, w, h - depth, r);
  ctx.stroke();

  // Inner glow
  ctx.shadowColor = s.glow;
  ctx.shadowBlur = size * 0.3;
  ctx.shadowOffsetY = 0;
  ctx.fillStyle = "transparent";
  roundRect(ctx, x + 3, y + 3, w - 6, h - depth - 6, r * 0.8);
  ctx.fill();
  ctx.shadowColor = "transparent";
  ctx.shadowBlur = 0;

  // Top surface highlight
  const grad = ctx.createLinearGradient(x, y, x, y + h - depth);
  grad.addColorStop(0, "rgba(255,255,255,0.08)");
  grad.addColorStop(0.5, "rgba(255,255,255,0.02)");
  grad.addColorStop(1, "rgba(0,0,0,0.05)");
  ctx.fillStyle = grad;
  roundRect(ctx, x, y, w, h - depth, r);
  ctx.fill();

  // Label
  const fontSize = label.length > 3 ? size * 0.25 : label.length > 1 ? size * 0.32 : size * 0.45;
  ctx.font = `600 ${fontSize}px "SF Mono", "Fira Code", "JetBrains Mono", "Cascadia Code", Consolas, monospace`;
  ctx.fillStyle = s.text;
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.fillText(label, x + w / 2, y + (h - depth) / 2 + 1);
}

function roundRect(ctx, x, y, w, h, r) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.lineTo(x + w - r, y);
  ctx.quadraticCurveTo(x + w, y, x + w, y + r);
  ctx.lineTo(x + w, y + h - r);
  ctx.quadraticCurveTo(x + w, y + h, x + w - r, y + h);
  ctx.lineTo(x + r, y + h);
  ctx.quadraticCurveTo(x, y + h, x, y + h - r);
  ctx.lineTo(x, y + r);
  ctx.quadraticCurveTo(x, y, x + r, y);
  ctx.closePath();
}

function KeycapPreview({ label, style, size, onClick }) {
  const canvasRef = useRef(null);
  const drawn = useRef(false);

  const refCb = useCallback(
    (node) => {
      canvasRef.current = node;
      if (node) {
        drawKeycap(node, label, style, size);
        drawn.current = true;
      }
    },
    [label, style, size]
  );

  const handleDownload = (e) => {
    e.stopPropagation();
    const c = canvasRef.current;
    if (!c) return;
    const link = document.createElement("a");
    link.download = `ghostspell-key-${label.toLowerCase().replace(/[^a-z0-9]/g, "")}-${style}.png`;
    link.href = c.toDataURL("image/png");
    link.click();
  };

  return (
    <div
      onClick={() => onClick && onClick(label)}
      style={{
        display: "inline-flex",
        flexDirection: "column",
        alignItems: "center",
        gap: 6,
        cursor: "pointer",
        position: "relative",
      }}
    >
      <canvas ref={refCb} />
      <button
        onClick={handleDownload}
        style={{
          background: "rgba(255,255,255,0.08)",
          border: "1px solid rgba(255,255,255,0.15)",
          color: "#aaa",
          fontSize: 11,
          padding: "2px 10px",
          borderRadius: 4,
          cursor: "pointer",
          fontFamily: "inherit",
        }}
      >
        PNG ↓
      </button>
    </div>
  );
}

export default function GhostSpellKeycapGen() {
  const [customLabel, setCustomLabel] = useState("G");
  const [style, setStyle] = useState("ghost");
  const [size, setSize] = useState(80);

  return (
    <div
      style={{
        minHeight: "100vh",
        background: "#0c0c14",
        color: "#d4d4d4",
        fontFamily: '"IBM Plex Mono", "Fira Code", monospace',
        padding: "32px 24px",
      }}
    >
      <div style={{ maxWidth: 900, margin: "0 auto" }}>
        {/* Header */}
        <div style={{ marginBottom: 32 }}>
          <h1
            style={{
              fontSize: 28,
              fontWeight: 700,
              color: "#c4b5fd",
              margin: 0,
              letterSpacing: "-0.02em",
            }}
          >
            GhostSpell Keycap Generator
          </h1>
          <p style={{ color: "#666", fontSize: 13, margin: "6px 0 0" }}>
            Generate keycap images on the fly — click any key or type your own
          </p>
        </div>

        {/* Controls */}
        <div
          style={{
            display: "flex",
            gap: 16,
            flexWrap: "wrap",
            marginBottom: 28,
            alignItems: "flex-end",
          }}
        >
          <div>
            <label style={{ fontSize: 11, color: "#888", display: "block", marginBottom: 4 }}>
              KEY LABEL
            </label>
            <input
              value={customLabel}
              onChange={(e) => setCustomLabel(e.target.value)}
              style={{
                background: "#1a1a2e",
                border: "1px solid #3a3a6a",
                color: "#e0e0e0",
                padding: "8px 14px",
                borderRadius: 6,
                fontSize: 16,
                width: 140,
                fontFamily: "inherit",
                outline: "none",
              }}
              placeholder="Type a key…"
            />
          </div>

          <div>
            <label style={{ fontSize: 11, color: "#888", display: "block", marginBottom: 4 }}>
              STYLE
            </label>
            <div style={{ display: "flex", gap: 6 }}>
              {Object.entries(STYLES).map(([key, s]) => (
                <button
                  key={key}
                  onClick={() => setStyle(key)}
                  style={{
                    background: style === key ? "#3a3a6a" : "#1a1a2e",
                    border: `1px solid ${style === key ? "#7c6fd4" : "#333"}`,
                    color: style === key ? "#e0d4ff" : "#888",
                    padding: "8px 14px",
                    borderRadius: 6,
                    fontSize: 12,
                    cursor: "pointer",
                    fontFamily: "inherit",
                    transition: "all 0.15s",
                  }}
                >
                  {s.label}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label style={{ fontSize: 11, color: "#888", display: "block", marginBottom: 4 }}>
              SIZE: {size}px
            </label>
            <input
              type="range"
              min={40}
              max={160}
              value={size}
              onChange={(e) => setSize(Number(e.target.value))}
              style={{ width: 120, accentColor: "#7c6fd4" }}
            />
          </div>
        </div>

        {/* Main preview */}
        <div
          style={{
            background: STYLES[style].bg,
            borderRadius: 12,
            padding: 40,
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
            marginBottom: 28,
            border: "1px solid rgba(255,255,255,0.06)",
            minHeight: 160,
          }}
        >
          {customLabel.trim() ? (
            <KeycapPreview
              key={`${customLabel}-${style}-${size}`}
              label={customLabel}
              style={style}
              size={size}
            />
          ) : (
            <span style={{ color: "#555" }}>Type a label above…</span>
          )}
        </div>

        {/* Preset grid */}
        <div>
          <label style={{ fontSize: 11, color: "#888", display: "block", marginBottom: 10 }}>
            PRESETS — click to preview, ↓ to download
          </label>
          <div
            style={{
              display: "flex",
              flexWrap: "wrap",
              gap: 12,
              background: STYLES[style].bg,
              borderRadius: 12,
              padding: 24,
              border: "1px solid rgba(255,255,255,0.06)",
            }}
          >
            {PRESETS.map((key) => (
              <KeycapPreview
                key={`${key}-${style}-${size}`}
                label={key}
                style={style}
                size={Math.min(size, 70)}
                onClick={(l) => setCustomLabel(l)}
              />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
