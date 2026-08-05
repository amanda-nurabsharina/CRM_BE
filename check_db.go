package main

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"time"
)

type Lead struct {
	ID           string `gorm:"type:uuid;primary_key"`
	CustomerName string
	PhoneNumber  string
	AvatarURL    string
	BranchID     *string
	CreatedAt    time.Time
}

type Conversation struct {
	ID            string `gorm:"type:uuid;primary_key"`
	LeadID        string
	BranchID      string
	LastMessageAt time.Time
}

type Message struct {
	ID             string `gorm:"type:uuid;primary_key"`
	ConversationID string
	SenderID       string
	Content        string
	Direction      string
	SentAt         time.Time
}

func main() {
	db, err := gorm.Open(sqlite.Open("crm.db"), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	var leads []Lead
	db.Find(&leads)
	fmt.Printf("=== LEADS (%d) ===\n", len(leads))
	for _, l := range leads {
		fmt.Printf("Lead: ID=%s Name=%s Phone=%s Avatar=%s BranchID=%v\n", l.ID, l.CustomerName, l.PhoneNumber, l.AvatarURL, l.BranchID)
	}

	var convs []Conversation
	db.Find(&convs)
	fmt.Printf("=== CONVERSATIONS (%d) ===\n", len(convs))
	for _, c := range convs {
		fmt.Printf("Conv: ID=%s LeadID=%s BranchID=%s LastMessageAt=%v\n", c.ID, c.LeadID, c.BranchID, c.LastMessageAt)
	}

	var msgs []Message
	db.Order("sent_at desc").Limit(10).Find(&msgs)
	fmt.Printf("=== RECENT MESSAGES (%d) ===\n", len(msgs))
	for _, m := range msgs {
		fmt.Printf("Msg: ConvID=%s Sender=%s Dir=%s Content=%s SentAt=%v\n", m.ConversationID, m.SenderID, m.Direction, m.Content, m.SentAt)
	}
}
