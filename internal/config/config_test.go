package config

import (
	"net/url"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"
)

func TestMysqlDSN(t *testing.T) {
	parsedURL, err := url.Parse("mysql://user:secret@db.example.com:3306/mydb?parseTime=true&charset=utf8mb4")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	dsn := mysqlDSN(parsedURL)

	cfg, err := gomysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("generated DSN %q is invalid: %v", dsn, err)
	}

	if cfg.User != "user" {
		t.Errorf("User = %q, want %q", cfg.User, "user")
	}
	if cfg.Passwd != "secret" {
		t.Errorf("Passwd = %q, want %q", cfg.Passwd, "secret")
	}
	if cfg.Addr != "db.example.com:3306" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, "db.example.com:3306")
	}
	if cfg.DBName != "mydb" {
		t.Errorf("DBName = %q, want %q", cfg.DBName, "mydb")
	}
	if !cfg.ParseTime {
		t.Error("ParseTime = false, want true")
	}
	if cfg.Params["charset"] != "utf8mb4" {
		t.Errorf("charset = %q, want %q", cfg.Params["charset"], "utf8mb4")
	}
}

func TestMysqlDSNParseTimeOverride(t *testing.T) {
	parsedURL, err := url.Parse("mysql://user:secret@db.example.com:3306/mydb?parseTime=false")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	cfg, err := gomysql.ParseDSN(mysqlDSN(parsedURL))
	if err != nil {
		t.Fatalf("generated DSN is invalid: %v", err)
	}

	if cfg.ParseTime {
		t.Error("ParseTime = true, want false")
	}
}

func TestValidateSessionKey(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config
		wantErr bool
	}{
		{"development allows the default key", config{AppEnv: "development", SessionKey_: defaultSessionKey}, false},
		{"production rejects the default key", config{AppEnv: "production", SessionKey_: defaultSessionKey}, true},
		{"production accepts a custom key", config{AppEnv: "production", SessionKey_: "s3cret-value"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
