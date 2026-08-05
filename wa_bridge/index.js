const { default: makeWASocket, useMultiFileAuthState, DisconnectReason } = require("@whiskeysockets/baileys");
const express = require("express");
const cors = require("cors");
const QRCode = require("qrcode");
const axios = require("axios");
const fs = require("fs");
const path = require("path");

const app = express();
app.use(cors());
app.use(express.json());

const PORT = process.env.PORT || 3001;
const CRM_WEBHOOK_URL = process.env.CRM_WEBHOOK_URL || "http://localhost:8000/v1/webhooks/whatsapp";

let sock = null;
let currentQR = "";
let connectionStatus = "DISCONNECTED";
let isSyncingHistory = false;
let syncedCount = 0;
const phoneToJidMap = {}; // Global JID cache
const lidToPhoneMap = {}; // Cache LID -> Real Phone number mapping
const avatarCache = {};
const contactNameMap = {};

async function getAvatarUrl(jid, phone) {
  if (!sock || connectionStatus !== "CONNECTED" || (!jid && !phone)) return "";
  let targetJid = jid || `${phone}@s.whatsapp.net`;
  if (phone && phone.length >= 5 && !phone.endsWith("lid") && !phone.includes("@")) {
    targetJid = `${phone}@s.whatsapp.net`;
  }

  try {
    if (avatarCache[targetJid]) return avatarCache[targetJid];
    let url = await sock.profilePictureUrl(targetJid, "image").catch(() => null);
    if (!url && targetJid !== jid && jid) {
      url = await sock.profilePictureUrl(jid, "image").catch(() => null);
    }
    if (url) {
      avatarCache[targetJid] = url;
      return url;
    }
    return "";
  } catch (e) {
    return "";
  }
}

function getRealPhoneNumber(msg) {
  if (!msg || !msg.key) return null;
  const fromJid = msg.key.remoteJid || "";

  // Ignore group chats (@g.us), channels (@newsletter), status broadcasts, and system bot JIDs
  if (
    !fromJid ||
    fromJid.endsWith("@g.us") ||
    fromJid.endsWith("@newsletter") ||
    fromJid.includes("status@broadcast") ||
    fromJid === "0@s.whatsapp.net"
  ) {
    return null;
  }

  let phone = "";

  // Try extracting standard WhatsApp phone number from alt fields or participant
  if (msg.key.remoteJidAlt && msg.key.remoteJidAlt.endsWith("@s.whatsapp.net")) {
    phone = msg.key.remoteJidAlt.split("@")[0];
  } else if (msg.key.participantAlt && msg.key.participantAlt.endsWith("@s.whatsapp.net")) {
    phone = msg.key.participantAlt.split("@")[0];
  } else if (msg.key.participant && msg.key.participant.endsWith("@s.whatsapp.net")) {
    phone = msg.key.participant.split("@")[0];
  } else if (msg.participant && msg.participant.endsWith("@s.whatsapp.net")) {
    phone = msg.participant.split("@")[0];
  }

  if (!phone) {
    if (fromJid.endsWith("@s.whatsapp.net")) {
      phone = fromJid.split("@")[0];
    } else if (fromJid.endsWith("@lid")) {
      const lidId = fromJid.split("@")[0];
      if (lidToPhoneMap[lidId]) {
        phone = lidToPhoneMap[lidId];
      }
    }
  }

  // Save mapping for outbound messaging
  if (fromJid.endsWith("@lid") && phone && !phone.endsWith("lid")) {
    const lidId = fromJid.split("@")[0];
    lidToPhoneMap[lidId] = phone;
    phoneToJidMap[phone] = fromJid;
  } else if (fromJid.endsWith("@s.whatsapp.net") && phone) {
    phoneToJidMap[phone] = fromJid;
  }

  // If phone is LID JID without resolved PN, fallback to LID ID so conversation is created
  if (!phone && fromJid.endsWith("@lid")) {
    phone = fromJid.split("@")[0];
  }

  const cleanDigits = phone ? phone.replace(/[^0-9]/g, "") : "";
  if (!cleanDigits || cleanDigits === "0" || cleanDigits.length < 5) {
    return null; // Skip invalid 0 or empty numbers
  }

  return cleanDigits;
}

async function connectToWhatsApp() {
  const { state, saveCreds } = await useMultiFileAuthState("auth_info_baileys");

  sock = makeWASocket({
    auth: state,
    syncFullHistory: true,
    markOnlineOnConnect: true,
    getMessage: async (key) => {
      return { conversation: "" };
    },
  });

  sock.ev.on("creds.update", saveCreds);

  sock.ev.on("connection.update", async (update) => {
    const { connection, lastDisconnect, qr } = update;

    if (qr) {
      currentQR = await QRCode.toDataURL(qr);
      connectionStatus = "PAIRING";
      console.log("[WA-BRIDGE] New QR Code generated, ready to scan!");
    }

    if (connection === "close") {
      const statusCode = (lastDisconnect?.error)?.output?.statusCode;
      const isLoggedOut = statusCode === DisconnectReason.loggedOut || statusCode === 401;
      console.log(`[WA-BRIDGE] Connection closed (statusCode: ${statusCode}). IsLoggedOut: ${isLoggedOut}...`);
      
      if (isLoggedOut) {
        console.log("[WA-BRIDGE] Device logged out or 401 error. Clearing old auth session...");
        connectionStatus = "DISCONNECTED";
        currentQR = "";
        try {
          fs.rmSync(path.join(__dirname, "auth_info_baileys"), { recursive: true, force: true });
        } catch (e) {}
      } else {
        connectionStatus = "CONNECTING";
      }

      setTimeout(() => {
        connectToWhatsApp();
      }, 1000);
    } else if (connection === "open") {
      console.log("[WA-BRIDGE] WhatsApp Real Connection Opened & Authenticated!");
      connectionStatus = "CONNECTED";
      currentQR = "";
    }
  });

  // Global exception catch to keep bridge alive
  process.on("uncaughtException", (err) => {
    console.error("[WA-BRIDGE] Uncaught Exception caught (keeping server running):", err.message);
  });

  process.on("unhandledRejection", (reason) => {
    console.error("[WA-BRIDGE] Unhandled Rejection caught (keeping server running):", reason);
  });

  // Helper function to extract human-readable text and ignore protocol packets
  function parseMessageContent(msg) {
    if (!msg || !msg.message) return null;
    
    // Ignore protocol packets, key distribution, reactions, app state sync
    if (
      msg.message.protocolMessage ||
      msg.message.senderKeyDistributionMessage ||
      msg.message.reactionMessage ||
      msg.message.peerDataOperationRequestMessage
    ) {
      return null;
    }

    let text =
      msg.message.conversation ||
      msg.message.extendedTextMessage?.text ||
      msg.message.imageMessage?.caption ||
      msg.message.videoMessage?.caption ||
      msg.message.documentMessage?.caption;

    if (!text) {
      if (msg.message.imageMessage) text = "[Gambar]";
      else if (msg.message.videoMessage) text = "[Video]";
      else if (msg.message.audioMessage) text = "[Pesan Suara]";
      else if (msg.message.documentMessage) text = "[Dokumen / File]";
      else if (msg.message.stickerMessage) text = "[Stiker]";
      else if (msg.message.contactMessage || msg.message.contactsArrayMessage) text = "[Kontak]";
      else if (msg.message.locationMessage || msg.message.liveLocationMessage) text = "[Lokasi]";
    }

    return text || null;
  }

  sock.ev.on("contacts.upsert", (contacts) => {
    for (const c of contacts) {
      if (c.id && (c.name || c.notify)) {
        const p = c.id.split("@")[0].replace(/[^0-9]/g, "");
        const name = c.name || c.notify;
        if (p && name) {
          contactNameMap[p] = name;
          contactNameMap[c.id] = name;
          if (c.id.endsWith("@s.whatsapp.net")) {
            phoneToJidMap[p] = c.id;
          }
        }
      }
    }
  });

  sock.ev.on("contacts.update", (updates) => {
    for (const c of updates) {
      if (c.id && (c.name || c.notify)) {
        const p = c.id.split("@")[0].replace(/[^0-9]/g, "");
        const name = c.name || c.notify;
        if (p && name) {
          contactNameMap[p] = name;
          contactNameMap[c.id] = name;
        }
      }
    }
  });

  sock.ev.on("chats.upsert", async (chats) => {
    for (const chat of chats) {
      const jid = chat.id || "";
      if (!jid || jid.endsWith("@g.us") || jid.endsWith("@newsletter") || jid.includes("status@broadcast")) continue;

      let phone = jid.split("@")[0].replace(/[^0-9]/g, "");
      if (jid.endsWith("@lid")) {
        const foundPN = Object.keys(phoneToJidMap).find((p) => phoneToJidMap[p] === jid);
        if (foundPN) phone = foundPN;
      }

      if (!phone || phone === "0" || phone.length < 5) continue;
      const name = contactNameMap[phone] || contactNameMap[jid] || chat.name || chat.pushName || `WA User (+${phone})`;
      const ts = chat.conversationTimestamp ? (typeof chat.conversationTimestamp === "number" ? chat.conversationTimestamp : chat.conversationTimestamp.low || 0) : 0;

      try {
        await axios.post(CRM_WEBHOOK_URL, {
          from_phone: phone,
          sender_name: name,
          content: "Sesi WhatsApp Terhubung (Utas Chat)",
          direction: "INBOUND",
          media_type: "TEXT",
          is_history: true,
          sent_at: ts,
        });
      } catch (err) {}
    }
  });

  // Handle Full WhatsApp History Sync when scanning QR code
  sock.ev.on("messaging-history.set", async ({ chats, contacts, messages, isLatest }) => {
    console.log(`[WA-BRIDGE] 📚 Historical Sync Received: ${messages ? messages.length : 0} past messages across ${chats ? chats.length : 0} chats! (isLatest: ${isLatest})`);
    isSyncingHistory = true;
    
    // Populate contact names
    if (contacts && contacts.length > 0) {
      contacts.forEach((c) => {
        if (c.id && (c.name || c.notify)) {
          const p = c.id.split("@")[0].replace(/[^0-9]/g, "");
          const name = c.name || c.notify;
          if (p && name) {
            contactNameMap[p] = name;
            contactNameMap[c.id] = name;
            phoneToJidMap[p] = c.id;
          }
        }
      });
    }

    // First, import all chat threads to populate Inbox sidebar
    if (chats && chats.length > 0) {
      for (const chat of chats) {
        const jid = chat.id || "";
        if (!jid || jid.endsWith("@g.us") || jid.endsWith("@newsletter") || jid.includes("status@broadcast")) continue;

        let phone = jid.split("@")[0].replace(/[^0-9]/g, "");
        if (jid.endsWith("@lid")) {
          const foundPN = Object.keys(phoneToJidMap).find((p) => phoneToJidMap[p] === jid);
          if (foundPN) phone = foundPN;
        }

        if (!phone || phone === "0" || phone.length < 5) continue;
        const name = contactNameMap[phone] || contactNameMap[jid] || chat.name || chat.pushName || `WA User (+${phone})`;
        const ts = chat.conversationTimestamp ? (typeof chat.conversationTimestamp === "number" ? chat.conversationTimestamp : chat.conversationTimestamp.low || 0) : 0;

        try {
          await axios.post(CRM_WEBHOOK_URL, {
            from_phone: phone,
            sender_name: name,
            content: "Sesi WhatsApp Terhubung (Riwayat Chat)",
            direction: "INBOUND",
            media_type: "TEXT",
            is_history: true,
            sent_at: ts,
          });
          syncedCount++;
        } catch (err) {}
      }
    }

    // Next, import all messages
    if (messages && messages.length > 0) {
      for (const msg of messages) {
        const phone = getRealPhoneNumber(msg);
        if (!phone) continue;

        const content = parseMessageContent(msg);
        if (!content) continue;

        const fromJid = msg.key?.remoteJid || `${phone}@s.whatsapp.net`;
        const isFromMe = msg.key.fromMe;
        const senderName = contactNameMap[phone] || contactNameMap[fromJid] || msg.pushName || `WA User (+${phone})`;
        const sentAt = msg.messageTimestamp ? (typeof msg.messageTimestamp === "number" ? msg.messageTimestamp : msg.messageTimestamp.low || 0) : 0;

        try {
          await axios.post(CRM_WEBHOOK_URL, {
            from_phone: phone,
            sender_name: senderName,
            content: content,
            direction: isFromMe ? "OUTBOUND" : "INBOUND",
            media_type: "TEXT",
            is_history: true,
            sent_at: sentAt,
          });
          syncedCount++;
        } catch (err) {}
      }
    }
    isSyncingHistory = false;
    console.log(`[WA-BRIDGE] 📚 Historical Sync import finished! Total synced: ${syncedCount}`);
  });

  sock.ev.on("messages.upsert", async (m) => {
    if (m.type === "notify" || m.type === "append") {
      for (const msg of m.messages) {
        if (msg.message) {
          const phone = getRealPhoneNumber(msg);
          if (!phone) continue;

          const content = parseMessageContent(msg);
          if (!content) continue; // Skip non-human packets

          const fromJid = msg.key?.remoteJid || `${phone}@s.whatsapp.net`;
          const avatarUrl = await getAvatarUrl(fromJid, phone);

          const senderName = contactNameMap[phone] || contactNameMap[fromJid] || msg.pushName || `WA User (+${phone})`;
          const isFromMe = msg.key.fromMe;
          const isHistory = m.type === "append" || isFromMe;
          const sentAt = msg.messageTimestamp ? (typeof msg.messageTimestamp === "number" ? msg.messageTimestamp : msg.messageTimestamp.low || 0) : 0;

          console.log(`[WA-BRIDGE] ${isFromMe ? "Outbound (from HP)" : "Inbound"} WA Message from ${phone} (${senderName}): ${content}`);

          // Forward to Go CRM Webhook
          try {
            await axios.post(CRM_WEBHOOK_URL, {
              from_phone: phone,
              sender_name: senderName,
              content: content,
              direction: isFromMe ? "OUTBOUND" : "INBOUND",
              media_type: "TEXT",
              is_history: isHistory,
              avatar_url: avatarUrl,
              sent_at: sentAt,
            });
            console.log(`[WA-BRIDGE] Successfully forwarded to CRM backend.`);
          } catch (err) {
            console.error(`[WA-BRIDGE] Error forwarding to CRM backend:`, err.message);
          }
        }
      }
    }
  });

  // Listen to Inbound WhatsApp Voice & Video Calls
  sock.ev.on("call", async (callEvents) => {
    for (const call of callEvents) {
      if (call.status === "offer") {
        const fromJid = call.from || "";
        if (fromJid.endsWith("@g.us")) continue;

        let phone = fromJid.split("@")[0];
        const isVideo = call.isVideo ? "Video" : "Suara";
        console.log(`[WA-BRIDGE] 📞 Inbound WhatsApp ${isVideo} Call OFFER from ${phone} (${fromJid})`);

        try {
          await axios.post(CRM_WEBHOOK_URL, {
            from_phone: phone,
            sender_name: `WhatsApp Call (+${phone})`,
            content: `📞 Panggilan WhatsApp ${isVideo} Masuk (Calling...)`,
            direction: "INBOUND",
            media_type: "CALL",
          });
          console.log(`[WA-BRIDGE] Successfully forwarded Inbound Call to CRM backend.`);
        } catch (err) {
          console.error(`[WA-BRIDGE] Error forwarding Inbound Call:`, err.message);
        }
      }
    }
  });
}

// API Endpoints
app.get("/status", (req, res) => {
  res.json({
    status: connectionStatus,
    qr_code_url: currentQR,
    is_syncing_history: isSyncingHistory,
    synced_count: syncedCount,
  });
});

app.get("/avatar", async (req, res) => {
  const phone = req.query.phone || "";
  const jid = req.query.jid || "";
  const url = await getAvatarUrl(jid, phone);
  res.json({ phone, avatar_url: url });
});

app.post("/reset", async (req, res) => {
  try {
    console.log("[WA-BRIDGE] 🔄 Reset, Unlink session & Clear Inbox requested...");
    if (sock) {
      if (connectionStatus === "CONNECTED") {
        try {
          await sock.logout().catch(() => null);
        } catch (e) {}
      }
      try {
        sock.end();
      } catch (e) {}
      sock = null;
    }
    connectionStatus = "DISCONNECTED";
    currentQR = "";
    isSyncingHistory = false;
    syncedCount = 0;

    const authDir = path.join(__dirname, "auth_info_baileys");
    if (fs.existsSync(authDir)) {
      fs.rmSync(authDir, { recursive: true, force: true });
    }

    // Immediately trigger WhatsApp connection for instant QR generation
    connectToWhatsApp();

    // Async clear inbox on CRM backend in background with 2s timeout
    const crmBackendBase = (process.env.CRM_WEBHOOK_URL || "http://crm-backend:8000")
      .replace(/\/v1\/webhooks\/whatsapp\/?$/, "")
      .replace(/\/$/, "");

    axios.post(`${crmBackendBase}/v1/admin/clear-inbox`, {}, { timeout: 2000 }).catch(() => null);

    return res.json({ status: "RESETTING", message: "WA Bridge unlinked and inbox cleared successfully" });
  } catch (err) {
    console.error("[WA-BRIDGE] Error resetting session:", err);
    return res.status(500).json({ error: err.message });
  }
});

app.post("/send", async (req, res) => {
  try {
    const { to, text } = req.body;
    if (!to || !text) {
      return res.status(400).json({ error: "Missing 'to' or 'text'" });
    }

    if (connectionStatus !== "CONNECTED" || !sock) {
      return res.status(503).json({ error: "WhatsApp bridge is not connected. Please scan QR code first." });
    }

    let cleanPhone = to.replace(/[^0-9]/g, "");
    if (cleanPhone.startsWith("0")) {
      cleanPhone = "62" + cleanPhone.slice(1);
    }

    if (!to.includes("@") && cleanPhone.length < 5) {
      return res.status(400).json({ error: "Invalid phone number: Must contain at least 5 digits" });
    }

    // Resolve target JIDs (support both LID format e.g. @lid and standard phone @s.whatsapp.net)
    let targetJids = [];
    if (phoneToJidMap[to] || phoneToJidMap[cleanPhone]) {
      targetJids.push(phoneToJidMap[to] || phoneToJidMap[cleanPhone]);
    } else if (to.includes("@")) {
      targetJids.push(to);
    } else if (cleanPhone.length >= 13) {
      targetJids.push(`${cleanPhone}@lid`);
      targetJids.push(`${cleanPhone}@s.whatsapp.net`);
    } else {
      targetJids.push(`${cleanPhone}@s.whatsapp.net`);
    }

    console.log(`[WA-BRIDGE] Sending outbound WA to targets [${targetJids.join(", ")}]: ${text}`);
    let sentCount = 0;
    for (const jid of targetJids) {
      try {
        await sock.sendMessage(jid, { text: text });
        console.log(`[WA-BRIDGE] Successfully delivered outbound WA message to: ${jid}`);
        sentCount++;
      } catch (err) {
        console.error(`[WA-BRIDGE] Error sending to JID ${jid}:`, err.message);
      }
    }

    return res.json({ status: "SENT", to: cleanPhone, deliveredCount: sentCount });
  } catch (err) {
    console.error("[WA-BRIDGE] Error sending WA message:", err);
    return res.status(500).json({ error: err.message });
  }
});

app.listen(PORT, "0.0.0.0", () => {
  console.log(`[WA-BRIDGE] Real WhatsApp Bridge running on http://0.0.0.0:${PORT}`);
  connectToWhatsApp();
});
