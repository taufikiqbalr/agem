package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsRawMongoURISecret(t *testing.T) {
	secretPath := writeTempSecret(t, "mongodb://mandor:secret@127.0.0.1:57215/kala_test?authSource=admin")
	clearMongoEnv(t)
	t.Setenv("MONGODB_SECRET_PATH", secretPath)

	cfg := Load()

	if cfg.MongoURI != "mongodb://mandor:secret@127.0.0.1:57215/kala_test?authSource=admin" {
		t.Fatalf("unexpected uri: %q", cfg.MongoURI)
	}
	if cfg.MongoDBName != "kala_test" {
		t.Fatalf("unexpected db name: %q", cfg.MongoDBName)
	}
}

func TestLoadReadsMongoURIFromEnvStyleSecret(t *testing.T) {
	secretPath := writeTempSecret(t, `
[mandor_agem]
MONGODB_URI="mongodb://mandor:secret@127.0.0.1:47215/kala_dev?authSource=admin"
`)
	clearMongoEnv(t)
	t.Setenv("MONGODB_SECRET_PATH", secretPath)

	cfg := Load()

	if cfg.MongoURI != "mongodb://mandor:secret@127.0.0.1:47215/kala_dev?authSource=admin" {
		t.Fatalf("unexpected uri: %q", cfg.MongoURI)
	}
	if cfg.MongoDBName != "kala_dev" {
		t.Fatalf("unexpected db name: %q", cfg.MongoDBName)
	}
}

func TestLoadBuildsMongoURIFromLegacySecretFields(t *testing.T) {
	secretPath := writeTempSecret(t, `
hosts=talas51.regene.xyz:57215
pass=secret
user=mandorKalaagem
db=regene_kalaagem
authSource=admin
`)
	clearMongoEnv(t)
	t.Setenv("MONGODB_SECRET_PATH", secretPath)

	cfg := Load()

	want := "mongodb://mandorKalaagem:secret@talas51.regene.xyz:57215/regene_kalaagem?authSource=admin"
	if cfg.MongoURI != want {
		t.Fatalf("unexpected uri: %q", cfg.MongoURI)
	}
	if cfg.MongoDBName != "regene_kalaagem" {
		t.Fatalf("unexpected db name: %q", cfg.MongoDBName)
	}
}

func TestLoadBuildsRegeneMongoURIFromNasabahSecret(t *testing.T) {
	secretPath := writeTempSecret(t, `
[nasabah_regene]
pass=secret
user=nasabahRegene
host=dalas1.regene.xyz
port=27017
db=regeneNode_dev
authSource=admin
tls=true
CAFile=/etc/gembok/pem_www/ca_dalas1_san.pem
CertificateKeyFile=/etc/gembok/pem_www/dalas1_san.pem
`)
	clearMongoEnv(t)
	t.Setenv("NASABAH_REGENE", secretPath)

	cfg := Load()

	want := "mongodb://nasabahRegene:secret@dalas1.regene.xyz:27017/regeneNode_dev?authSource=admin&tls=true&tlsCAFile=%2Fetc%2Fgembok%2Fpem_www%2Fca_dalas1_san.pem&tlsCertificateKeyFile=%2Fetc%2Fgembok%2Fpem_www%2Fdalas1_san.pem"
	if cfg.RegeneMongoURI != want {
		t.Fatalf("unexpected regene uri: %q", cfg.RegeneMongoURI)
	}
	if cfg.RegeneMongoDBName != "regeneNode_dev" {
		t.Fatalf("unexpected regene db name: %q", cfg.RegeneMongoDBName)
	}
	if cfg.RegeneUsersCollection != "users" {
		t.Fatalf("unexpected regene users collection: %q", cfg.RegeneUsersCollection)
	}
}

func TestLoadPrefersExplicitMongoDBName(t *testing.T) {
	clearMongoEnv(t)
	t.Setenv("MONGODB_URI", "mongodb://mandor:secret@127.0.0.1:57215/kala_from_uri?authSource=admin")
	t.Setenv("MONGODB_DB", "kala_override")

	cfg := Load()

	if cfg.MongoDBName != "kala_override" {
		t.Fatalf("unexpected db name: %q", cfg.MongoDBName)
	}
}

func TestLoadUsesRegeneKalaagemFallbackDatabase(t *testing.T) {
	clearMongoEnv(t)

	cfg := Load()

	if cfg.MongoDBName != "regene_kalaagem" {
		t.Fatalf("unexpected db name: %q", cfg.MongoDBName)
	}
}

func clearMongoEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ADDR",
		"PORT",
		"MONGODB_URI",
		"MONGO_URI",
		"MONGODB_URI_FILE",
		"MONGODB_SECRET_PATH",
		"MONGODB_DB",
		"DB_NAME",
		"REGENE_MONGODB_URI",
		"REGENE_MONGO_URI",
		"NASABAH_REGENE_URI",
		"REGENE_MONGODB_URI_FILE",
		"REGENE_MONGODB_SECRET_PATH",
		"NASABAH_REGENE",
		"REGENE_MONGODB_DB",
		"REGENE_DB",
		"REGENE_USERS_COLLECTION",
	} {
		t.Setenv(key, "")
	}
}

func writeTempSecret(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mandor_agem")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
