const { default: makeWASocket, useMultiFileAuthState, DisconnectReason } = require("@whiskeysockets/baileys");
const express = require("express");
const cors = require("cors");
const QRCode = require("qrcode");
const axios = require("axios");

const app = express();
app.use(cors());
app.use(express.json());

const PORT = 3001;
const CRM_WEBHOOK_URL = "http://localhost:8000/v1/webhooks/whatsapp";

let sock = null;
let currentQR = "";
let connectionStatus = "DISCONNECTED";
const phoneToJidMap = {}; // Global JID cache

async function connectToWhatsApp() {
  const { state, saveCreds } = await useMultiFileAuthState("auth_info_baileys");

  sock = makeWASocket({
    auth: state,
    printQRInTerminal: true,
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
      const shouldReconnect = (lastDisconnect?.error)?.output?.statusCode !== DisconnectReason.loggedOut;
      console.log("[WA-BRIDGE] Connection closed due to ", lastDisconnect?.error, ", reconnecting: ", shouldReconnect);
      connectionStatus = "DISCONNECTED";
      currentQR = "";
      if (shouldReconnect) {
        connectToWhatsApp();
      }
    } else if (connection === "open") {
      console.log("[WA-BRIDGE] WhatsApp Real Connection Opened & Authenticated!");
      connectionStatus = "CONNECTED";
      currentQR = "";
    }
  });

  sock.ev.on("messages.upsert", async (m) => {
    if (m.type === "notify") {
      for (const msg of m.messages) {
        if (msg.message) {
          const fromJid = msg.key.remoteJid || "";

          // Ignore group chats (@g.us), channels (@newsletter), and broadcast status
          if (fromJid.endsWith("@g.us") || fromJid.endsWith("@newsletter") || fromJid.includes("status@broadcast")) {
            continue;
          }

          let phone = fromJid.split("@")[0];
          if (fromJid.endsWith("@lid")) {
            if (msg.key.participant && msg.key.participant.endsWith("@s.whatsapp.net")) {
              phone = msg.key.participant.split("@")[0];
            } else if (msg.participant && msg.participant.endsWith("@s.whatsapp.net")) {
              phone = msg.participant.split("@")[0];
            }
          }

          phoneToJidMap[phone] = fromJid; // Save mapping for outbound delivery
          if (fromJid.endsWith("@lid")) {
            phoneToJidMap[fromJid.split("@")[0]] = fromJid;
          }

          const senderName = msg.pushName || `WA User (${phone})`;

          const content =
            msg.message.conversation ||
            msg.message.extendedTextMessage?.text ||
            msg.message.imageMessage?.caption ||
            msg.message.videoMessage?.caption ||
            msg.message.documentMessage?.caption ||
            "[Pesan Media/Gambar]";

          const isFromMe = msg.key.fromMe;

          console.log(`[WA-BRIDGE] ${isFromMe ? "Outbound (from HP)" : "Inbound"} WA Message from ${phone} (${fromJid}): ${content}`);

          // Forward to Go CRM Webhook
          try {
            await axios.post(CRM_WEBHOOK_URL, {
              from_phone: phone,
              sender_name: senderName,
              content: content,
              direction: isFromMe ? "OUTBOUND" : "INBOUND",
              media_type: "TEXT",
            });
            console.log(`[WA-BRIDGE] Successfully forwarded to CRM backend.`);
          } catch (err) {
            console.error(`[WA-BRIDGE] Error forwarding to CRM backend:`, err.message);
          }
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
  });
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

app.listen(PORT, () => {
  console.log(`[WA-BRIDGE] Real WhatsApp Bridge running on http://localhost:${PORT}`);
  connectToWhatsApp();
});
