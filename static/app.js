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

/** Names we used when /api/join-defaults failed; do not treat as a chosen nickname on reload. */
function isGenericPlaceholderName(name) {
  const s = (name || "").trim();
  return s === "Player" || s === "Guest";
}

function resolveDisplayNameFromProfile(profile, defaults) {
  const prev =
    profile && profile.name && validateName(profile.name) ? profile.name.trim() : "";
  if (prev && !isGenericPlaceholderName(prev)) return prev;
  return defaults.name;
}

/** Server uses the same dictionary-backed generators as /join when fields are invalid or empty. */
async function fetchJoinDefaultsLoose() {
  try {
    const res = await fetch("/api/join-defaults");
    if (!res.ok) throw new Error("bad status");
    const body = await res.json();
    if (typeof body.name !== "string" || !body.name.trim()) throw new Error("bad name");
    return {
      name: body.name.trim(),
      room: typeof body.room === "string" ? body.room.trim() : "",
    };
  } catch {
    return { name: "Guest", room: "" };
  }
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
    <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
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
    <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
      <circle cx="12" cy="12" r="3"></circle>
    </svg>
  `;
}

function heroIconPencil() {
  return `
    <svg class="icon icon--participant-edit" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M12 20h9"></path>
      <path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"></path>
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

async function joinPageInit() {
  setView("welcome");
  renderIconInto("join-button-icon", heroIconJoin());

  const profile = readProfile();
  const nameInput = $("name-input");
  const roomInput = $("room-input");
  const joinBtn = $("join-button");
  const joinError = $("join-error");

  if (joinBtn) joinBtn.disabled = true;
  const defaults = await fetchJoinDefaultsLoose();
  if (joinBtn) joinBtn.disabled = false;

  const fallbackName = defaults.name;
  const savedName =
    profile && profile.name && validateName(profile.name) ? profile.name.trim() : "";
  const useSavedName = savedName && !isGenericPlaceholderName(savedName);
  nameInput.value = useSavedName ? savedName : fallbackName;
  if (roomInput && defaults.room && isValidRoomSlug(defaults.room)) {
    roomInput.value = defaults.room;
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
      saveLocal(data.uuid, data.token, data.name || name);
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
      const d = await fetchJoinDefaultsLoose();
      const displayName = resolveDisplayNameFromProfile(profile, d);
      const body = await postJoin(roomKey, displayName, uuid);
      saveLocal(body.uuid, body.token, body.name || displayName);
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
  const cardsEditInput = $("cards-edit-input");
  const cardsEditButton = $("cards-edit-button");

  if (revealButton) {
    revealButton.innerHTML = `${heroIconReveal()}<span class="sr-only">Reveal</span>Reveal cards`;
  }

  let lastState = null;
  let editingMyName = false;
  let myNameDraft = "";
  let suppressParticipantBlur = false;
  let focusMyNameAfterRender = false;

  function cancelMyNameEdit() {
    editingMyName = false;
    if (lastState) renderParticipants(lastState);
  }

  function tryCommitMyName() {
    const v = myNameDraft.trim();
    if (!validateName(v)) return;
    sendWS({ type: "name_update", name: v });
    editingMyName = false;
    if (lastState) renderParticipants(lastState);
  }

  function renderParticipants(state) {
    suppressParticipantBlur = true;
    playersList.innerHTML = "";
    try {
      const players = inRoomPlayers(state);
      participantsCount.textContent = `${players.length}`;
      const revealed = !!state.revealed;

      for (const p of players) {
        const row = document.createElement("div");
        row.className = "participant-row";

        const left = document.createElement("div");
        left.className = "participant-row-left";

        if (p.uuid === effectiveUUID) {
          const slot = document.createElement("div");
          slot.className = "participant-name-edit-slot";

          if (editingMyName) {
            const inp = document.createElement("input");
            inp.className = "participant-name-input";
            inp.type = "text";
            inp.autocomplete = "off";
            inp.setAttribute("aria-label", "Your display name");
            inp.value = myNameDraft;
            inp.addEventListener("input", () => {
              myNameDraft = inp.value;
            });
            inp.addEventListener("keydown", (e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                tryCommitMyName();
              } else if (e.key === "Escape") {
                e.preventDefault();
                cancelMyNameEdit();
              }
            });
            inp.addEventListener("blur", () => {
              if (suppressParticipantBlur) return;
              tryCommitMyName();
            });
            slot.appendChild(inp);
          } else {
            const wrap = document.createElement("div");
            wrap.className = "participant-name-with-edit";

            const nameSpan = document.createElement("div");
            nameSpan.className = "participant-name";
            nameSpan.textContent = p.name || "Unknown";

            const editBtn = document.createElement("button");
            editBtn.type = "button";
            editBtn.className = "btn-participant-name-edit";
            editBtn.setAttribute("aria-label", "Edit your name");
            editBtn.innerHTML = heroIconPencil();
            editBtn.addEventListener("click", () => {
              editingMyName = true;
              myNameDraft = (p.name || "").trim();
              focusMyNameAfterRender = true;
              if (lastState) renderParticipants(lastState);
            });

            wrap.appendChild(nameSpan);
            wrap.appendChild(editBtn);
            slot.appendChild(wrap);
          }

          left.appendChild(slot);
        } else {
          const nameSpan = document.createElement("div");
          nameSpan.className = "participant-name";
          nameSpan.textContent = p.name || "Unknown";
          left.appendChild(nameSpan);
        }

        const votedBadge = document.createElement("div");
        votedBadge.className =
          "badge-vote " + (p.voted ? "badge-vote--on" : "badge-vote--off");
        votedBadge.textContent = revealed ? (p.card || "—") : (p.voted ? "Voted" : "Not yet");

        row.appendChild(left);
        row.appendChild(votedBadge);
        playersList.appendChild(row);
      }
    } finally {
      suppressParticipantBlur = false;
    }

    if (editingMyName && focusMyNameAfterRender) {
      focusMyNameAfterRender = false;
      const inp = playersList.querySelector(".participant-name-input");
      if (inp) {
        requestAnimationFrame(() => {
          inp.focus();
          inp.select();
        });
      }
    }
  }

  function renderCards(state) {
    cardsList.innerHTML = "";
    const cards = state.cards || [];
    const revealed = !!state.revealed;
    const me = (state.players || []).find((x) => x.uuid === effectiveUUID);
    const selected = me && me.card ? me.card : null;

    for (const card of cards) {
      const btn = document.createElement("button");
      btn.type = "button";
      const longLabel = String(card).length > 3;
      const sizeClass = longLabel ? "card-vote--sm" : "card-vote--lg";

      let stateClass = "card-vote--default";
      if (revealed) {
        stateClass = "card-vote--revealed";
      } else if (selected && selected === card) {
        stateClass = "card-vote--selected";
      }

      btn.className = `card-vote ${sizeClass} ${stateClass}`;
      btn.textContent = card;

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

    const radius = 120; // tuned for .seat-circle (20rem)
    const cx = 160; // center in px (320px circle)
    const cy = 160;

    for (let i = 0; i < n; i++) {
      const p = players[i];
      const angle = (i / n) * Math.PI * 2 - Math.PI / 2;
      const x = cx + radius * Math.cos(angle);
      const y = cy + radius * Math.sin(angle);

      const seat = document.createElement("div");
      seat.dataset.seat = "1";
      seat.className = "seat-node";
      seat.style.left = `${x}px`;
      seat.style.top = `${y}px`;

      const name = document.createElement("div");
      name.className = "seat-name";
      name.textContent = p.name;

      const dot = document.createElement("div");
      dot.className = "seat-dot " + (p.voted ? "seat-dot--voted" : "seat-dot--idle");

      seat.appendChild(name);
      seat.appendChild(dot);
      seatCircle.appendChild(seat);
    }
  }

  function setRevealButton(state) {
    if (!revealButton) return;
    if (state.revealed) {
      revealButton.disabled = false;
      revealButton.innerHTML = `${heroIconReveal()}<span class="sr-only">New voting</span>Start new voting`;
      revealButton.onclick = () => sendWS({ type: "new_voting" });
      return;
    }
    revealButton.innerHTML = `${heroIconReveal()}<span class="sr-only">Reveal</span>Reveal cards`;
    // Always allow manual reveal; server also auto-reveals when everyone has voted.
    revealButton.disabled = false;
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
      const d = await fetchJoinDefaultsLoose();
      const prevName = resolveDisplayNameFromProfile(p, d);
      const body = await postJoin(roomKey, prevName, p?.uuid || "");
      saveLocal(body.uuid, body.token, body.name || prevName);
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
    void joinPageInit();
  }
}

document.addEventListener("DOMContentLoaded", () => {
  routeInit();
});

