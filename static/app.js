/* global WebSocket */

const STORAGE_KEY = "scrumpoke";

function $(id) {
  return document.getElementById(id);
}

function parseJwtExp(token) {
  // JWT is base64url encoded: header.payload.signature
  try {
    const parts = token.split(".");
    if (parts.length < 2) return null;
    let payload = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    // Add padding for atob.
    while (payload.length % 4) payload += "=";
    const json = atob(payload);
    const obj = JSON.parse(json);
    if (typeof obj.exp !== "number") return null;
    return obj.exp * 1000;
  } catch {
    return null;
  }
}

/**
 * Read stored profile (uuid + optional name + token). Does not delete storage when the JWT expires,
 * so we can refresh the token and keep the same display name.
 */
function readProfile() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (!parsed || !parsed.uuid) return null;
    const token = typeof parsed.token === "string" ? parsed.token : "";
    const name = typeof parsed.name === "string" ? parsed.name : "";
    let exp = parsed.expiresAt;
    if ((exp == null || exp === "") && token) exp = parseJwtExp(token);
    const tokenValid = !!(token && exp && Date.now() <= exp + 5000);
    return { uuid: parsed.uuid, token, name, expiresAt: exp || null, tokenValid };
  } catch {
    return null;
  }
}

/** @deprecated use readProfile; kept for minimal churn */
function loadLocal() {
  const p = readProfile();
  if (!p || !p.tokenValid) return null;
  return { uuid: p.uuid, token: p.token, expiresAt: p.expiresAt };
}

/**
 * Persist session. Pass `name` to set display name; omit to keep the previous name in storage.
 */
function saveLocal(uuid, token, name) {
  let prevName = "";
  try {
    const p = readProfile();
    if (p) prevName = p.name || "";
  } catch {
    /* ignore */
  }
  const finalName = name !== undefined && name !== null ? String(name) : prevName;
  const expiresAt = token ? parseJwtExp(token) : null;
  localStorage.setItem(
    STORAGE_KEY,
    JSON.stringify({
      uuid,
      token,
      name: finalName,
      expiresAt: expiresAt || null,
    }),
  );
}

function saveLocalName(name) {
  const p = readProfile();
  if (!p || !p.uuid) return;
  saveLocal(p.uuid, p.token, name);
}

async function postJoin(room, name, uuid) {
  const res = await fetch("/join", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ room, name, uuid: uuid || "" }),
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(body.error || `Join failed (${res.status})`);
  }
  return body;
}

/**
 * Open WebSocket; rejects if the socket closes before open (e.g. 401 on handshake) or times out.
 */
function openWebSocketWhenReady(roomKey, tok) {
  return new Promise((resolve, reject) => {
    const socket = connectRoomWS(roomKey, tok);
    let opened = false;
    const failTimer = setTimeout(() => {
      try {
        socket.close();
      } catch {
        /* ignore */
      }
      if (!opened) reject(new Error("timeout"));
    }, 8000);
    socket.onopen = () => {
      opened = true;
      clearTimeout(failTimer);
      resolve(socket);
    };
    socket.onclose = () => {
      if (!opened) {
        clearTimeout(failTimer);
        reject(new Error("auth"));
      }
    };
    socket.onerror = () => {
      /* onclose usually follows */
    };
  });
}

function randInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

function randomPlayerName() {
  const adjs = ["Scrum", "Poke", "Swift", "Brave", "Calm", "Turbo", "Quick", "Dapper"];
  const a = adjs[randInt(0, adjs.length - 1)];
  const n = randInt(1000, 9999);
  return `${a} ${n}`;
}

function randomRoomSlug() {
  const chars = "abcdefghijklmnopqrstuvwxyz0123456789";
  const part = (len) => {
    let s = "";
    for (let i = 0; i < len; i++) s += chars[randInt(0, chars.length - 1)];
    return s;
  };
  return `${part(6)}-${part(4)}`;
}

function validateName(name) {
  // letters, digits and spaces only
  if (name == null) return false;
  const trimmed = name.trim();
  if (!trimmed) return false;
  return /^[A-Za-z0-9 ]+$/.test(trimmed);
}

function validateCardsText(text) {
  // backend will validate; here is just a quick UX check
  return (text || "").split(",").map((s) => s.trim()).some(Boolean);
}

// Matches backend room slug rules ([a-z0-9-], 1–64 chars).
function isValidRoomSlug(s) {
  return typeof s === "string" && /^[a-z0-9-]{1,64}$/.test(s);
}

/** Players currently connected (shown as “in the room”). */
function inRoomPlayers(state) {
  return (state.players || []).filter((p) => p && p.connected);
}

function heroIconJoin() {
  // Heroicons-style outline "user plus" (inline SVG)
  return `
    <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"></path>
      <circle cx="9" cy="7" r="4"></circle>
      <line x1="19" y1="8" x2="19" y2="14"></line>
      <line x1="16" y1="11" x2="22" y2="11"></line>
    </svg>
  `;
}

function heroIconReveal() {
  // Eye icon
  return `
    <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
      <circle cx="12" cy="12" r="3"></circle>
    </svg>
  `;
}

function renderIconInto(id, html) {
  const el = $(id);
  if (!el) return;
  el.innerHTML = html;
}

function setView(view) {
  const welcome = $("welcome-view");
  const room = $("room-view");
  if (welcome) welcome.classList.toggle("hidden", view !== "welcome");
  if (room) room.classList.toggle("hidden", view !== "room");
}

function joinPageInit() {
  setView("welcome");
  renderIconInto("join-button-icon", heroIconJoin());

  const profile = readProfile();
  const nameInput = $("name-input");
  const roomInput = $("room-input");
  const joinBtn = $("join-button");
  const joinError = $("join-error");

  const fallbackName = randomPlayerName();
  if (profile && profile.name && validateName(profile.name)) {
    nameInput.value = profile.name.trim();
  } else {
    nameInput.value = fallbackName;
  }

  const uuid = profile ? profile.uuid : "";

  joinBtn.addEventListener("click", async () => {
    joinError.classList.add("hidden");
    joinError.textContent = "";

    const name = nameInput.value.trim() || fallbackName;
    const room = roomInput.value.trim(); // optional

    // Frontend validation is UX only; backend validates too.
    if (!validateName(name)) {
      joinError.textContent = "Name can only contain letters, digits, and spaces.";
      joinError.classList.remove("hidden");
      return;
    }

    const payload = { room, name, uuid };
    try {
      const res = await fetch("/join", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || `Join failed (${res.status})`);
      }
      const data = await res.json();
      saveLocal(data.uuid, data.token, name);
      window.location.href = `/room/${encodeURIComponent(data.room)}`;
    } catch (err) {
      joinError.textContent = String(err?.message || err || "Join failed");
      joinError.classList.remove("hidden");
    }
  });
}

function connectRoomWS(roomSlug, token) {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  const url = `${proto}//${location.host}/room/${encodeURIComponent(roomSlug)}/ws?token=${encodeURIComponent(token)}`;
  return new WebSocket(url);
}

async function roomViewInit() {
  setView("room");

  const wsError = $("room-error");

  const pathParts = window.location.pathname.split("/").filter(Boolean);
  // Expect /room/<slug> only (route is registered as /room/:room).
  if (pathParts[0] !== "room" || !pathParts[1]) {
    if (wsError) {
      wsError.textContent = "Invalid room link.";
      wsError.classList.remove("hidden");
    }
    return;
  }
  const rawSlug = pathParts[1];
  let roomKey = "";
  try {
    roomKey = decodeURIComponent(rawSlug || "").toLowerCase();
  } catch {
    roomKey = (rawSlug || "").toLowerCase();
  }

  // UUID lives only in localStorage, never in the URL. Strip legacy ?user= from bookmarks.
  if (new URLSearchParams(window.location.search).has("user")) {
    window.history.replaceState({}, "", window.location.pathname);
  }

  if (!isValidRoomSlug(roomKey)) {
    if (wsError) {
      wsError.textContent = "Invalid room link.";
      wsError.classList.remove("hidden");
    }
    return;
  }

  let profile = readProfile();
  let token = "";
  let effectiveUUID = "";

  try {
    if (profile && profile.tokenValid) {
      token = profile.token;
      effectiveUUID = profile.uuid;
    } else {
      const uuid = profile?.uuid || "";
      const prevName =
        profile && profile.name && validateName(profile.name)
          ? profile.name.trim()
          : "";
      const displayName = prevName || randomPlayerName();
      const body = await postJoin(roomKey, displayName, uuid);
      saveLocal(body.uuid, body.token, displayName);
      token = body.token;
      effectiveUUID = body.uuid;
      if (body.room && body.room !== roomKey) {
        window.history.replaceState({}, "", `/room/${encodeURIComponent(body.room)}`);
        roomKey = body.room;
      }
    }
  } catch (err) {
    if (wsError) {
      wsError.textContent =
        String(err?.message || err || "Could not join this room. Try the home page.");
      wsError.classList.remove("hidden");
    }
    return;
  }

  // Elements
  const playersList = $("players-list");
  const participantsCount = $("participants-count");
  const cardsList = $("cards-list");
  const revealButton = $("reveal-button");
  const roomNameInput = $("room-name-input");
  const cardsEditInput = $("cards-edit-input");
  const cardsEditButton = $("cards-edit-button");

  if (revealButton) revealButton.innerHTML = `${heroIconReveal()} <span class="sr-only">Reveal</span>Reveal cards`;

  let lastState = null;
  let myNameDraft = "";

  function renderParticipants(state) {
    playersList.innerHTML = "";
    const players = inRoomPlayers(state);
    participantsCount.textContent = `${players.length}`;
    const revealed = !!state.revealed;

    for (const p of players) {
      const row = document.createElement("div");
      row.className = "flex items-center justify-between gap-2";

      const left = document.createElement("div");
      left.className = "min-w-0";

      const nameSpan = document.createElement("div");
      nameSpan.className = "truncate text-sm";
      nameSpan.textContent = p.name || "Unknown";

      const votedBadge = document.createElement("div");
      votedBadge.className = "text-xs rounded-full px-2 py-0.5 " + (p.voted ? "bg-emerald-500/20 text-emerald-200" : "bg-white/10 text-white/60");
      votedBadge.textContent = revealed ? (p.card || "—") : (p.voted ? "Voted" : "Not yet");

      left.appendChild(nameSpan);
      row.appendChild(left);
      row.appendChild(votedBadge);
      playersList.appendChild(row);
    }

    if (roomNameInput && state && state.players) {
      const me = state.players.find((x) => x.uuid === effectiveUUID);
      if (me) {
        // Avoid overriding while the user is typing.
        if (!roomNameInput.matches(":focus")) {
          roomNameInput.value = me.name || "";
        }
      }
    }
  }

  function renderCards(state) {
    cardsList.innerHTML = "";
    const cards = state.cards || [];
    const revealed = !!state.revealed;

    const cardFrame =
      "inline-flex items-center justify-center shrink-0 rounded-xl border-2 text-center leading-tight px-1.5 " +
      "w-14 h-[5.5rem] sm:w-16 sm:h-[6.25rem] transition select-none font-bold ";

    for (const card of cards) {
      const btn = document.createElement("button");
      btn.type = "button";
      const longLabel = String(card).length > 3;
      const typeScale = longLabel ? "text-sm sm:text-base " : "text-lg sm:text-xl ";

      let face = cardFrame + typeScale;
      if (revealed) {
        face +=
          "border-white/20 bg-white/[0.04] text-white/35 cursor-not-allowed opacity-90";
      } else {
        face +=
          "border-white/20 bg-gradient-to-b from-slate-800/95 to-slate-950 text-slate-100 " +
          "hover:from-slate-700 hover:to-slate-900" +
          "hover:-translate-y-0.5 active:translate-y-0 cursor-pointer";
      }

      btn.className = face;
      btn.textContent = card;

      // Highlight selected card for myself pre-reveal.
      const me = (state.players || []).find((x) => x.uuid === effectiveUUID);
      const selected = me && me.card ? me.card : null;
      if (!revealed && selected && selected === card) {
        btn.className =
          cardFrame +
          typeScale +
          "border-emerald-500/45 bg-gradient-to-b from-emerald-950/90 to-slate-950 text-emerald-100 " +
          "ring-2 ring-emerald-500/35 cursor-pointer " +
          "hover:from-emerald-900/80 hover:to-slate-950 hover:border-emerald-400/55 " +
          "hover:-translate-y-0.5 active:translate-y-0";
      }

      btn.addEventListener("click", () => {
        if (lastState && lastState.revealed) return;
        sendWS({ type: "vote", card });
      });

      cardsList.appendChild(btn);
    }

    if (cardsEditInput && state) {
      // Keep editable; user can overwrite it.
      cardsEditInput.value = cards.join(", ");
    }
  }

  function renderSeats(state) {
    const seatCircle = $("seat-circle");
    if (!seatCircle) return;

    // Remove any existing seat elements.
    const existing = seatCircle.querySelectorAll("[data-seat]");
    existing.forEach((x) => x.remove());

    const players = inRoomPlayers(state);
    const n = players.length;
    if (n === 0) return;

    const radius = 120; // tuned for w-80/h-80
    const cx = 160; // center in px (w-80 => 320px)
    const cy = 160;

    for (let i = 0; i < n; i++) {
      const p = players[i];
      const angle = (i / n) * Math.PI * 2 - Math.PI / 2;
      const x = cx + radius * Math.cos(angle);
      const y = cy + radius * Math.sin(angle);

      const seat = document.createElement("div");
      seat.dataset.seat = "1";
      seat.className = "absolute -translate-x-1/2 -translate-y-1/2 w-32 text-center";
      seat.style.left = `${x}px`;
      seat.style.top = `${y}px`;

      const name = document.createElement("div");
      name.className = "text-xs font-semibold truncate px-1";
      name.textContent = p.name;

      const dot = document.createElement("div");
      dot.className =
        "mt-1 mx-auto w-2.5 h-2.5 rounded-full " +
        (p.voted ? "bg-emerald-400" : "bg-white/30");

      seat.appendChild(name);
      seat.appendChild(dot);
      seatCircle.appendChild(seat);
    }
  }

  function setRevealButton(state) {
    if (!revealButton) return;
    if (state.revealed) {
      revealButton.disabled = false;
      revealButton.innerHTML = `${heroIconReveal()} Start new voting`;
      revealButton.onclick = () => sendWS({ type: "new_voting" });
      return;
    }
    revealButton.innerHTML = `${heroIconReveal()} Reveal cards`;
    // Always allow manual reveal; server also auto-reveals when everyone has voted.
    revealButton.disabled = false;
    revealButton.classList.remove("cursor-not-allowed", "opacity-60");
    revealButton.onclick = () => sendWS({ type: "reveal" });
  }

  function renderState(state) {
    lastState = state;
    const me = (state.players || []).find((x) => x.uuid === effectiveUUID);
    if (me && me.name && validateName(me.name)) {
      saveLocalName(me.name.trim());
    }
    renderParticipants(state);
    renderCards(state);
    renderSeats(state);
    setRevealButton(state);
  }

  function parseStateMessage(msg) {
    if (!msg || msg.type !== "state") return null;
    return msg;
  }

  function showRoomError(text) {
    if (!wsError) return;
    wsError.textContent = text;
    wsError.classList.remove("hidden");
  }

  let ws = null;
  function sendWS(obj) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(JSON.stringify(obj));
  }

  try {
    ws = await openWebSocketWhenReady(roomKey, token);
  } catch {
    try {
      const p = readProfile();
      const prevName =
        p && p.name && validateName(p.name) ? p.name.trim() : randomPlayerName();
      const body = await postJoin(roomKey, prevName, p?.uuid || "");
      saveLocal(body.uuid, body.token, prevName);
      token = body.token;
      effectiveUUID = body.uuid;
      if (body.room && body.room !== roomKey) {
        window.history.replaceState({}, "", `/room/${encodeURIComponent(body.room)}`);
        roomKey = body.room;
      }
      ws = await openWebSocketWhenReady(roomKey, token);
    } catch (e2) {
      showRoomError(
        String(e2?.message || e2 || "Could not connect. Try refreshing the page."),
      );
      return;
    }
  }

  if (wsError) wsError.classList.add("hidden");
  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data);
      const state = parseStateMessage(msg);
      if (state) renderState(state);
    } catch {
      // ignore
    }
  };
  ws.onerror = () => {
    /* browser may fire before onclose */
  };
  ws.onclose = () => {
    showRoomError("Disconnected from room.");
  };

  // Name editing
  if (roomNameInput) {
    roomNameInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        if (!validateName(roomNameInput.value)) return;
        sendWS({ type: "name_update", name: roomNameInput.value.trim() });
      }
    });
    roomNameInput.addEventListener("blur", () => {
      if (!validateName(roomNameInput.value)) return;
      sendWS({ type: "name_update", name: roomNameInput.value.trim() });
    });
  }

  // Cards editing
  if (cardsEditButton) {
    cardsEditButton.addEventListener("click", () => {
      if (!cardsEditInput) return;
      const txt = cardsEditInput.value.trim();
      if (!validateCardsText(txt)) return;
      sendWS({ type: "cards_update", cards: txt });
    });
  }

  // Preload cards edit input with current cards after first state.
  // (We update on render.)
}

function routeInit() {
  const path = window.location.pathname;
  if (path.startsWith("/room/")) {
    void roomViewInit();
  } else {
    joinPageInit();
  }
}

document.addEventListener("DOMContentLoaded", () => {
  routeInit();
});

