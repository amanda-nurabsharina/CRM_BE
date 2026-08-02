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
        if (!msg.key.fromMe && msg.message) {
          const fromJid = msg.key.remoteJid;
          const phone = fromJid.split("@")[0];
          const senderName = msg.pushName || `WA User (${phone})`;
          const content = msg.message.conversation || msg.message.extendedTextMessage?.text || "[Media Message]";

          console.log(`[WA-BRIDGE] Inbound WA Message from ${phone} (${senderName}): ${content}`);

          // Forward to Go CRM Webhook
          try {
            await axios.post(CRM_WEBHOOK_URL, {
              from_phone: phone,
              sender_name: senderName,
              content: content,
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
    const formattedJid = `${cleanPhone}@s.whatsapp.net`;

    console.log(`[WA-BRIDGE] Sending outbound WA to ${formattedJid}: ${text}`);
    await sock.sendMessage(formattedJid, { text: text });

    return res.json({ status: "SENT", to: cleanPhone });
  } catch (err) {
    console.error("[WA-BRIDGE] Error sending WA message:", err);
    return res.status(500).json({ error: err.message });
  }
});

app.listen(PORT, () => {
  console.log(`[WA-BRIDGE] Real WhatsApp Bridge running on http://localhost:${PORT}`);
  connectToWhatsApp();
});
