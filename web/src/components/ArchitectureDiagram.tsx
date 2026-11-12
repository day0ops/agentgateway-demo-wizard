import { useEffect, useRef, useState } from "react";
import { onThemeChange } from "../lib/theme";
import { PaneLabel } from "./PaneLabel";

// Hand-drawn HTML5 canvas overview of the demo's security architecture: every
// call crosses agentgateway, the single policy-enforcement point, which is
// drawn as a pipeline of controls rather than an opaque box. Trust zones,
// the identity provider, and the delegation identity are made explicit so the
// welcome screen reads as a security map, not just a topology. Colors are read
// from the design tokens each render, so it tracks the active theme. Hovering an
// element shows a tooltip (hit-tested against regions registered during draw).

// Authored in a fixed coordinate space and scaled to the container width.
const BASE_W = 1200;
const BASE_H = 640;

// A hit-testable element, with the tooltip shown on hover. Coordinates are in
// the fixed BASE space, so they survive scaling.
type Hotspot = {
  x: number;
  y: number;
  w: number;
  h: number;
  title: string;
  body: string;
};

type Tip = {
  title: string;
  body: string;
  left: number;
  top: number;
  below: boolean;
};

// Tooltip copy, grounded in what each element actually does in the demo.
const TIP = {
  caller:
    "The calling user or application. Its requests always go to agentgateway with an identity token - never straight to an agent or tool.",
  idp: "The OIDC identity provider. agentgateway signs users in here, and validates - and exchanges - the tokens it issues for delegation.",
  authN:
    "Verifies who is calling before anything else runs: ext-auth redirects to Keycloak over OIDC and validates the returned token.",
  authZ:
    "Decides what the identity may do - including per-MCP-tool RBAC and the scopes a delegated call is allowed to use.",
  guardrails:
    "Inspects request and response content: blocks banned phrases and masks PII such as SSNs, account, and card numbers.",
  routing:
    "Routes each request to the right model or backend, attributes token cost per team, and enforces budgets via virtual keys.",
  accounts:
    "Answers account questions by reading Core Banking on the caller's behalf (impersonation).",
  transfer:
    "Coordinates a transfer and delegates settlement to the Settlement agent - routed through the gateway, not a direct call.",
  settlement:
    "Settles the transfer against Core Banking, permitted only for the right task within the delegation chain.",
  statements:
    "Builds statements and reaches the PayPal MCP using interactive elicitation.",
  coreBanking:
    "The bank's internal core system - balances, transfers, settlement - reachable only through the gateway.",
  paypal:
    "Third-party payment service reached over the internet. Tool calls go through interactive elicitation.",
};

// Screen-reader equivalent of the drawn diagram.
const A11Y_LABEL =
  "Architecture overview. Every call flows through agentgateway, the single " +
  "policy enforcement point, which applies authentication, authorization and " +
  "RBAC, content guardrails, and routing and cost controls. The caller and the " +
  "four banking agents - Accounts, Transfer Orchestrator, Settlement, and " +
  "Statements - reach it, and agent-to-tool and agent-to-agent delegation calls " +
  "re-enter it before reaching the MCP backends: the internal Core Banking MCP " +
  "and the external, third-party PayPal MCP server. agentgateway " +
  "verifies and " +
  "exchanges identity tokens with the Keycloak identity provider, carrying the " +
  "delegation chain sub, act, act.act.";

export function ArchitectureDiagram() {
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const hotspotsRef = useRef<Hotspot[]>([]);
  const hoveredRef = useRef<Hotspot | null>(null);
  const [tip, setTip] = useState<Tip | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container) return;

    function render() {
      if (!canvas || !container) return;
      const ctx = canvas.getContext("2d");
      if (!ctx) return;

      const cssW = Math.min(container.clientWidth, 1240);
      const cssH = cssW * (BASE_H / BASE_W);
      const dpr = window.devicePixelRatio || 1;
      canvas.width = Math.round(cssW * dpr);
      canvas.height = Math.round(cssH * dpr);
      // Pin the display size to the backing store (capped width, centered), so a
      // wide container can't stretch the fixed-width bitmap horizontally.
      canvas.style.width = `${cssW}px`;
      canvas.style.height = `${cssH}px`;
      ctx.setTransform(
        (dpr * cssW) / BASE_W,
        0,
        0,
        (dpr * cssH) / BASE_H,
        0,
        0,
      );
      ctx.clearRect(0, 0, BASE_W, BASE_H);
      const hotspots: Hotspot[] = [];
      drawDiagram(ctx, hotspots);
      hotspotsRef.current = hotspots;
    }

    render();
    // Redraw once the Geist webfont is ready, on theme change, and on resize,
    // so text and token colors always match the current state.
    document.fonts?.ready?.then(render).catch(() => {});
    const unsubscribe = onThemeChange(render);
    const observer = new ResizeObserver(render);
    observer.observe(container);
    return () => {
      unsubscribe();
      observer.disconnect();
    };
  }, []);

  function handleMove(e: React.MouseEvent<HTMLCanvasElement>) {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container) return;
    const rect = canvas.getBoundingClientRect();
    const scaleX = rect.width / BASE_W;
    const scaleY = rect.height / BASE_H;
    const bx = (e.clientX - rect.left) / scaleX;
    const by = (e.clientY - rect.top) / scaleY;

    // Topmost (last-registered) matching region wins, so nodes beat the zones
    // and band they sit inside.
    const regions = hotspotsRef.current;
    let found: Hotspot | null = null;
    for (let i = regions.length - 1; i >= 0; i--) {
      const r = regions[i];
      if (bx >= r.x && bx <= r.x + r.w && by >= r.y && by <= r.y + r.h) {
        found = r;
        break;
      }
    }
    if (found === hoveredRef.current) return;
    hoveredRef.current = found;
    canvas.style.cursor = found ? "pointer" : "default";
    if (!found) {
      setTip(null);
      return;
    }
    // Anchor the tooltip to the element (centered, above it - or below when the
    // element is near the top edge), clamped within the container.
    const halfW = 132;
    const centerX = canvas.offsetLeft + (found.x + found.w / 2) * scaleX;
    const left = Math.max(
      halfW + 4,
      Math.min(centerX, container.clientWidth - halfW - 4),
    );
    const topEdge = canvas.offsetTop + found.y * scaleY;
    const bottomEdge = canvas.offsetTop + (found.y + found.h) * scaleY;
    const below = topEdge < 96;
    setTip({
      title: found.title,
      body: found.body,
      left,
      top: below ? bottomEdge + 10 : topEdge - 10,
      below,
    });
  }

  function handleLeave() {
    hoveredRef.current = null;
    if (canvasRef.current) canvasRef.current.style.cursor = "default";
    setTip(null);
  }

  return (
    <div className="px-6 pb-8">
      <div className="mb-3">
        <PaneLabel>Architecture overview</PaneLabel>
      </div>
      <div
        ref={containerRef}
        className="relative rounded-xl border border-border bg-card p-4 shadow-card"
      >
        <canvas
          ref={canvasRef}
          role="img"
          aria-label={A11Y_LABEL}
          onMouseMove={handleMove}
          onMouseLeave={handleLeave}
          className="mx-auto block max-w-full"
        />
        {tip && (
          <div
            className="pointer-events-none absolute z-20 w-64 rounded-lg border border-border bg-card p-3 shadow-elevated"
            style={{
              left: tip.left,
              top: tip.top,
              transform: tip.below
                ? "translate(-50%, 0)"
                : "translate(-50%, -100%)",
            }}
          >
            <div className="text-sm font-semibold text-foreground">
              {tip.title}
            </div>
            <div className="mt-1 text-xs leading-relaxed text-muted-foreground">
              {tip.body}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function drawDiagram(ctx: CanvasRenderingContext2D, hotspots: Hotspot[]) {
  const cs = getComputedStyle(document.documentElement);
  const v = (name: string) => `rgb(${cs.getPropertyValue(name).trim()})`;
  const va = (name: string, a: number) =>
    `rgb(${cs.getPropertyValue(name).trim()} / ${a})`;

  const c = {
    surface: v("--card"),
    elevated: v("--elevated"),
    border: v("--border"),
    borderStrong: v("--border-strong"),
    fg: v("--foreground"),
    muted: v("--muted-foreground"),
    subtle: v("--subtle-foreground"),
    primary: v("--primary"),
    primarySoft: va("--primary", 0.08),
    primaryLine: va("--primary", 0.55),
    primaryFg: v("--primary-foreground"),
    info: v("--info"),
    infoSoft: va("--info", 0.1),
    warning: v("--warning"),
    warningSoft: va("--warning", 0.09),
    warningLine: va("--warning", 0.4),
    zoneFill: va("--muted-foreground", 0.045),
  };

  const FONT = '"Geist Sans", ui-sans-serif, system-ui, sans-serif';
  const MONO = '"Geist Mono", ui-monospace, monospace';

  function rr(x: number, y: number, w: number, h: number, r: number) {
    ctx.beginPath();
    ctx.roundRect(x, y, w, h, r);
  }

  function text(
    str: string,
    x: number,
    y: number,
    opts: {
      size?: number;
      weight?: number;
      color?: string;
      align?: CanvasTextAlign;
      family?: string;
    } = {},
  ) {
    const {
      size = 15,
      weight = 500,
      color = c.fg,
      align = "left",
      family = FONT,
    } = opts;
    ctx.font = `${weight} ${size}px ${family}`;
    ctx.fillStyle = color;
    ctx.textAlign = align;
    ctx.textBaseline = "middle";
    ctx.fillText(str, x, y);
  }

  // A rounded node with a centered title and optional sublabel.
  function node(
    x: number,
    y: number,
    w: number,
    h: number,
    opts: {
      title: string;
      sub?: string;
      fill?: string;
      border?: string;
      titleColor?: string;
      titleSize?: number;
      tip?: string;
    },
  ) {
    const {
      title,
      sub,
      fill = c.surface,
      border = c.border,
      titleColor = c.fg,
      titleSize = 15,
      tip,
    } = opts;
    rr(x, y, w, h, 12);
    ctx.fillStyle = fill;
    ctx.fill();
    ctx.lineWidth = 1.5;
    ctx.strokeStyle = border;
    ctx.stroke();
    const cx = x + w / 2;
    if (sub) {
      text(title, cx, y + h / 2 - 10, {
        align: "center",
        weight: 600,
        size: titleSize,
        color: titleColor,
      });
      text(sub, cx, y + h / 2 + 10, {
        align: "center",
        size: 12,
        color: c.muted,
      });
    } else {
      text(title, cx, y + h / 2, {
        align: "center",
        weight: 600,
        size: titleSize,
        color: titleColor,
      });
    }
    if (tip) hotspots.push({ x, y, w, h, title, body: tip });
  }

  // A tinted trust-zone container with a small eyebrow label.
  function zone(
    x: number,
    y: number,
    w: number,
    h: number,
    label: string,
    opts: { fill?: string; border?: string; labelColor?: string } = {},
  ) {
    const {
      fill = c.zoneFill,
      border = c.border,
      labelColor = c.subtle,
    } = opts;
    rr(x, y, w, h, 16);
    ctx.fillStyle = fill;
    ctx.fill();
    ctx.lineWidth = 1;
    ctx.setLineDash([5, 4]);
    ctx.strokeStyle = border;
    ctx.stroke();
    ctx.setLineDash([]);
    text(label.toUpperCase(), x + 16, y + 16, {
      size: 10,
      weight: 600,
      color: labelColor,
    });
  }

  function arrow(
    x1: number,
    y1: number,
    x2: number,
    y2: number,
    opts: {
      color?: string;
      width?: number;
      dashed?: boolean;
    } = {},
  ) {
    const { color = c.borderStrong, width = 2, dashed = false } = opts;
    ctx.strokeStyle = color;
    ctx.fillStyle = color;
    ctx.lineWidth = width;
    ctx.setLineDash(dashed ? [6, 5] : []);
    ctx.beginPath();
    ctx.moveTo(x1, y1);
    ctx.lineTo(x2, y2);
    ctx.stroke();
    ctx.setLineDash([]);
    const angle = Math.atan2(y2 - y1, x2 - x1);
    const head = 9;
    ctx.beginPath();
    ctx.moveTo(x2, y2);
    ctx.lineTo(
      x2 - head * Math.cos(angle - Math.PI / 6),
      y2 - head * Math.sin(angle - Math.PI / 6),
    );
    ctx.lineTo(
      x2 - head * Math.cos(angle + Math.PI / 6),
      y2 - head * Math.sin(angle + Math.PI / 6),
    );
    ctx.closePath();
    ctx.fill();
  }

  // A pill-backed label that sits legibly on top of an edge.
  function edgeLabel(str: string, x: number, y: number, mono = false) {
    ctx.font = `500 12px ${mono ? MONO : FONT}`;
    const padX = 8;
    const w = ctx.measureText(str).width + padX * 2;
    const h = 22;
    rr(x - w / 2, y - h / 2, w, h, 7);
    ctx.fillStyle = c.surface;
    ctx.fill();
    ctx.lineWidth = 1;
    ctx.strokeStyle = c.border;
    ctx.stroke();
    text(str, x, y, {
      size: 12,
      color: mono ? c.primary : c.muted,
      align: "center",
      family: mono ? MONO : FONT,
    });
  }

  function stepBadge(n: number, x: number, y: number) {
    ctx.beginPath();
    ctx.arc(x, y, 11, 0, Math.PI * 2);
    ctx.fillStyle = c.primary;
    ctx.fill();
    text(String(n), x, y + 0.5, {
      size: 12,
      weight: 700,
      color: c.primaryFg,
      align: "center",
    });
  }

  // --- Left: caller + identity provider -------------------------------------
  node(44, 96, 164, 54, { title: "You / app", sub: "caller", tip: TIP.caller });
  node(44, 300, 164, 58, {
    title: "Keycloak",
    sub: "identity provider",
    border: c.info,
    tip: TIP.idp,
  });

  // --- Center: the gateway as an enforcement pipeline -----------------------
  const gx = 318;
  const gy = 64;
  const gw = 250;
  const gh = 512;
  ctx.save();
  ctx.shadowColor = va("--primary", 0.28);
  ctx.shadowBlur = 26;
  rr(gx, gy, gw, gh, 18);
  ctx.fillStyle = c.primarySoft;
  ctx.fill();
  ctx.restore();
  rr(gx, gy, gw, gh, 18);
  ctx.lineWidth = 1.75;
  ctx.strokeStyle = c.primaryLine;
  ctx.stroke();
  text("agentgateway", gx + gw / 2, gy + 30, {
    align: "center",
    weight: 700,
    size: 18,
    color: c.fg,
  });
  text("single policy enforcement point", gx + gw / 2, gy + 52, {
    align: "center",
    size: 12,
    color: c.muted,
  });

  const stages: [string, string, string][] = [
    ["authN", "ext-auth · OIDC", TIP.authN],
    ["authZ · RBAC", "identity + tool scopes", TIP.authZ],
    ["guardrails", "PII · prompt content", TIP.guardrails],
    ["routing · cost", "budgets · attribution", TIP.routing],
  ];
  const stageX = gx + 22;
  const stageW = gw - 44;
  const stageH = 66;
  // Center the four stages as a group so the band isn't top-weighted.
  let sy = gy + 116;
  for (const [title, sub, tip] of stages) {
    rr(stageX, sy, stageW, stageH, 10);
    ctx.fillStyle = c.elevated;
    ctx.fill();
    ctx.lineWidth = 1;
    ctx.strokeStyle = c.border;
    ctx.stroke();
    // teal tick
    rr(stageX + 12, sy + stageH / 2 - 9, 3, 18, 2);
    ctx.fillStyle = c.primary;
    ctx.fill();
    text(title, stageX + 26, sy + stageH / 2 - 9, {
      weight: 600,
      size: 14,
    });
    text(sub, stageX + 26, sy + stageH / 2 + 10, {
      size: 11.5,
      color: c.muted,
    });
    hotspots.push({ x: stageX, y: sy, w: stageW, h: stageH, title, body: tip });
    sy += stageH + 24;
  }

  // --- Right-top: agents (workload trust zone) ------------------------------
  zone(676, 64, 480, 232, "Agents · workloads");
  const agents = [
    ["Accounts", 696, 110, TIP.accounts],
    ["Transfer Orchestrator", 926, 110, TIP.transfer],
    ["Settlement", 696, 196, TIP.settlement],
    ["Statements", 926, 196, TIP.statements],
  ] as const;
  for (const [name, ax, ay, tip] of agents) {
    node(ax, ay, 210, 64, { title: name, titleSize: 14, tip });
  }

  // --- Right-bottom: MCP backends, split by the internal/external boundary --
  // Core Banking and PayPal each sit centered in their zone (zone center,
  // y=456), so the two zones line up.
  zone(676, 336, 230, 240, "Internal");
  node(696, 428, 190, 56, { title: "Core Banking MCP", tip: TIP.coreBanking });

  zone(926, 336, 230, 240, "External", {
    fill: c.warningSoft,
    border: c.warningLine,
    labelColor: c.warning,
  });
  node(946, 428, 190, 56, { title: "PayPal MCP", tip: TIP.paypal });

  // --- Edges: every tier connects only to the gateway ----------------------
  // Clean perpendicular connectors into the central band; the gateway is the
  // one thing everything plugs into, so agent->tool and agent->agent mediation
  // is stated in the caption rather than drawn as a tangled return arrow.
  const gxR = gx + gw;

  // 1. caller -> gateway
  arrow(208, 123, gx, 123, { color: c.primary, width: 2.25 });
  edgeLabel("request + token", 263, 99);
  stepBadge(1, 263, 123);

  // gateway <-> IdP (verify / exchange token)
  arrow(gx, 330, 210, 330, { color: c.info, width: 1.75, dashed: true });
  edgeLabel("verify / exchange", 263, 330);

  // 2. gateway -> agents (north-south: carries the caller's identity)
  arrow(gxR, 180, 674, 180, { color: c.primary, width: 2.25 });
  edgeLabel("forwarded as the caller", 622, 154);
  stepBadge(2, 622, 180);

  // 3. gateway -> backends (east-west: agent-initiated tool/delegation calls)
  arrow(gxR, 456, 674, 456, { color: c.primary, width: 2.25 });
  edgeLabel("tool RBAC · delegation · elicitation", 626, 430);
  stepBadge(3, 622, 456);
  edgeLabel("sub → act → act.act", 621, 500, true);

  text(
    "Agents reach tools - and each other - only through the gateway",
    BASE_W / 2,
    616,
    { align: "center", size: 12.5, color: c.subtle },
  );
}
