const sqlite3 = require('sqlite3');
const path = require('path');

const dbPath = path.join(__dirname, 'crm.db');
const db = new sqlite3.Database(dbPath);

db.serialize(() => {
  db.run("DELETE FROM leads WHERE phone_number = '0' OR phone_number LIKE '692735%' OR phone_number LIKE '208636%' OR length(phone_number) < 5", (err) => {
    if (err) console.error("Error deleting leads:", err);
    else console.log("Deleted garbage leads.");
  });

  db.run("DELETE FROM conversations WHERE lead_id NOT IN (SELECT id FROM leads)", (err) => {
    if (err) console.error("Error deleting conversations:", err);
    else console.log("Deleted orphaned conversations.");
  });
});

db.close();
