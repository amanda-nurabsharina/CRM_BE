package database

import (
	"crm-be/src/config"
	"crm-be/src/model"
	"crm-be/src/utils"
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect() *gorm.DB {
	var db *gorm.DB
	var err error

	if config.DBDriver == "postgres" {
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
			config.DBHost, config.DBUser, config.DBPassword, config.DBName, config.DBPort, config.DBSslMode)
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			utils.Log.Warnf("Failed to connect to Postgres (%v), falling back to SQLite", err)
			db, err = gorm.Open(sqlite.Open(config.DBName), &gorm.Config{})
		}
	} else {
		db, err = gorm.Open(sqlite.Open(config.DBName), &gorm.Config{})
	}

	if err != nil {
		utils.Log.Fatalf("Failed to initialize database: %v", err)
	}

	utils.Log.Info("Database connection established successfully")

	// Auto Migrate
	if err := db.AutoMigrate(
		&model.Branch{},
		&model.User{},
		&model.RefreshToken{},
		&model.Lead{},
		&model.Conversation{},
		&model.Message{},
		&model.TourPackage{},
		&model.Quotation{},
		&model.Invoice{},
		&model.PaymentTerm{},
		&model.PaymentProof{},
		&model.BookingTraveler{},
		&model.TravelerDocument{},
		&model.AuditLog{},
	); err != nil {
		utils.Log.Errorf("Auto migration error: %v", err)
	} else {
		utils.Log.Info("Database auto migration completed for all CRM entities")
	}

	// Seed default DGT data if empty
	SeedDefaultData(db)

	return db
}

func SeedDefaultData(db *gorm.DB) {
	// Seed Branches
	var branchCount int64
	db.Model(&model.Branch{}).Count(&branchCount)
	if branchCount == 0 {
		branches := []model.Branch{
			{Name: "DGT Kantor Pusat", Code: "PUSAT", WAPhoneNumber: "628110001000", CoverageAreas: "Pusat, General, Indonesia, All, Default Fallback"},
			{Name: "DGT Jakarta Pusat", Code: "JKT_PST", WAPhoneNumber: "628110001001", CoverageAreas: "Jakarta Pusat, Gambir, Tanah Abang, Menteng, Senen, Cempaka Putih, Johar Baru, Kemayoran"},
			{Name: "DGT Jakarta Selatan", Code: "JKT_SEL", WAPhoneNumber: "628110001002", CoverageAreas: "Jakarta Selatan, Kebayoran Baru, Kebayoran Lama, Cilandak, Pesanggrahan, Pasar Minggu, Jagakarsa, Mampang Prapatan, Pancoran, Tebet, Setiabudi"},
			{Name: "DGT Jakarta Utara", Code: "JKT_UTR", WAPhoneNumber: "628110001003", CoverageAreas: "Jakarta Utara, Penjaringan, Pademangan, Tanjung Priok, Koja, Kelapa Gading, Cilincing"},
			{Name: "DGT Medan", Code: "MDN", WAPhoneNumber: "628110001004", CoverageAreas: "Medan, Sumatera Utara, Binjai, Deli Serdang"},
			{Name: "DGT Tangerang", Code: "TGR", WAPhoneNumber: "628110001005", CoverageAreas: "Tangerang, Gading Serpong, BSD, Karawaci, Alam Sutera"},
		}
		for i := range branches {
			db.Create(&branches[i])
		}
		utils.Log.Info("DGT Branches seeded successfully")
	}

	// Always ensure PUSAT branch exists
	var pusatCount int64
	db.Model(&model.Branch{}).Where("code = ?", "PUSAT").Count(&pusatCount)
	if pusatCount == 0 {
		pusatBranch := model.Branch{
			Name:          "DGT Kantor Pusat",
			Code:          "PUSAT",
			WAPhoneNumber: "628110001000",
			CoverageAreas: "Pusat, General, Indonesia, All, Default Fallback",
			IsActive:      true,
		}
		db.Create(&pusatBranch)
		utils.Log.Info("DGT Kantor Pusat branch seeded successfully")
	}

	// Purge legacy junk leads created from status broadcast or group JIDs
	db.Exec("DELETE FROM leads WHERE phone_number IN ('status', '120363379468732783') OR length(phone_number) > 17")
	db.Exec("DELETE FROM conversations WHERE lead_id NOT IN (SELECT id FROM leads)")

	// Seed Users
	var userCount int64
	db.Model(&model.User{}).Count(&userCount)
	if userCount == 0 {
		hashedPass, _ := utils.HashPassword("password123")

		var jktPusatBranch model.Branch
		db.Where("code = ?", "JKT_PST").First(&jktPusatBranch)

		var mdnBranch model.Branch
		db.Where("code = ?", "MDN").First(&mdnBranch)

		adminPusat := model.User{
			Name:     "Admin Pusat DGT",
			Email:    "pusat@dgt.co.id",
			Password: hashedPass,
			Role:     "ADMIN_PUSAT",
			IsActive: true,
		}
		db.Create(&adminPusat)

		adminJkt := model.User{
			Name:     "Admin Cabang Jakarta Pusat",
			Email:    "admin.jkt@dgt.co.id",
			Password: hashedPass,
			Role:     "ADMIN_CABANG",
			BranchID: &jktPusatBranch.ID,
			IsActive: true,
		}
		db.Create(&adminJkt)

		adminMdn := model.User{
			Name:     "Admin Cabang Medan",
			Email:    "admin.mdn@dgt.co.id",
			Password: hashedPass,
			Role:     "ADMIN_CABANG",
			BranchID: &mdnBranch.ID,
			IsActive: true,
		}
		db.Create(&adminMdn)

		utils.Log.Info("Default DGT CRM users seeded: pusat@dgt.co.id, admin.jkt@dgt.co.id, admin.mdn@dgt.co.id (Password: 'password123')")
	}

	// Seed Tour Packages
	var pkgCount int64
	db.Model(&model.TourPackage{}).Count(&pkgCount)
	if pkgCount == 0 {
		pkgs := []model.TourPackage{
			{
				Title:           "Japan Golden Route 7D6N (Tokyo, Mt. Fuji, Kyoto, Osaka)",
				Destination:     "Jepang",
				DurationDays:    7,
				BasePrice:       19500000,
				ItineraryJSON:   `[{"day":1,"title":"Arrival Tokyo"},{"day":2,"title":"Asakusa & Shibuya"},{"day":3,"title":"Mt. Fuji"},{"day":4,"title":"Shinkansen to Kyoto"},{"day":5,"title":"Arashiyama & Fushimi Inari"},{"day":6,"title":"Osaka Castle & Dotonbori"},{"day":7,"title":"Departure Osaka"}]`,
				TermsConditions: "Harga termasuk tiket pesawat PP, hotel bintang 4, JR Pass, dan guide lokal Bahasa Indonesia.",
				IsActive:        true,
			},
			{
				Title:           "Europe Highlights 10D9N (Paris, Brussels, Amsterdam, Frankfurt, Zurich)",
				Destination:     "Eropa Barat",
				DurationDays:    10,
				BasePrice:       34500000,
				ItineraryJSON:   `[{"day":1,"title":"Jakarta - Paris"},{"day":2,"title":"Paris City Tour"},{"day":3,"title":"Louvre & Eiffel"},{"day":4,"title":"Paris - Brussels - Amsterdam"},{"day":5,"title":"Zaanse Schans"},{"day":6,"title":"Amsterdam - Cologne"},{"day":7,"title":"Frankfurt - Heidelberg"},{"day":8,"title":"Titisee - Rhine Falls"},{"day":9,"title":"Zurich - Lucerne"},{"day":10,"title":"Zurich - Jakarta"}]`,
				TermsConditions: "Termasuk Schengen Visa assistance, insurance, bus pariwisata AC private, tour leader dari Jakarta.",
				IsActive:        true,
			},
			{
				Title:           "Korea Spectacular Winter/Spring 6D5N (Seoul, Nami Island)",
				Destination:     "Korea Selatan",
				DurationDays:    6,
				BasePrice:       12800000,
				ItineraryJSON:   `[{"day":1,"title":"Jakarta - Incheon"},{"day":2,"title":"Nami Island & Petite France"},{"day":3,"title":"Everland Theme Park"},{"day":4,"title":"Gyeongbokgung Palace & Myeongdong"},{"day":5,"title":"Hongdae & N Seoul Tower"},{"day":6,"title":"Incheon - Jakarta"}]`,
				TermsConditions: "Harga termasuk visa Korea group/individual, makan sesuai itinerary, hotel bintang 3+.",
				IsActive:        true,
			},
		}
		for i := range pkgs {
			db.Create(&pkgs[i])
		}
		utils.Log.Info("Default Tour Packages seeded successfully")
	}
}

