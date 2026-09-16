package config

import (
	"bufio"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	Addr                  string
	MongoURI              string
	MongoDBName           string
	RegeneMongoURI        string
	RegeneMongoDBName     string
	RegeneUsersCollection string
	ReadTimeout           time.Duration
	WriteTimeout          time.Duration
	IdleTimeout           time.Duration
}

func Load() Config {
	addr := getenv("ADDR", "")
	if addr == "" {
		addr = ":" + getenv("PORT", "8080")
	}

	mongoURI := getenv("MONGODB_URI", getenv("MONGO_URI", ""))
	if mongoURI == "" {
		mongoURI = readSecretValue(getenv("MONGODB_URI_FILE", ""))
	}
	if mongoURI == "" {
		mongoURI = readSecretValue(getenv("MONGODB_SECRET_PATH", ""))
	}

	dbName := getenv("MONGODB_DB", getenv("DB_NAME", ""))
	if dbName == "" {
		dbName = databaseNameFromURI(mongoURI)
	}
	if dbName == "" {
		dbName = "regene_kalaagem"
	}

	regeneURI := getenv("REGENE_MONGODB_URI", getenv("REGENE_MONGO_URI", getenv("NASABAH_REGENE_URI", "")))
	if regeneURI == "" {
		regeneURI = readSecretValue(getenv("REGENE_MONGODB_URI_FILE", ""))
	}
	if regeneURI == "" {
		regeneURI = readSecretValue(getenv("REGENE_MONGODB_SECRET_PATH", ""))
	}
	if regeneURI == "" {
		regeneURI = readSecretValue(getenv("NASABAH_REGENE", ""))
	}

	regeneDBName := getenv("REGENE_MONGODB_DB", getenv("REGENE_DB", ""))
	if regeneDBName == "" {
		regeneDBName = databaseNameFromURI(regeneURI)
	}

	regeneUsersCollection := getenv("REGENE_USERS_COLLECTION", "users")

	return Config{
		Addr:                  addr,
		MongoURI:              mongoURI,
		MongoDBName:           dbName,
		RegeneMongoURI:        regeneURI,
		RegeneMongoDBName:     regeneDBName,
		RegeneUsersCollection: regeneUsersCollection,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          20 * time.Second,
		IdleTimeout:           60 * time.Second,
	}
}

func getenv(k, def string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	return v
}

func readSecretValue(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "mongodb://") || strings.HasPrefix(raw, "mongodb+srv://") {
		return raw
	}

	values := parseEnvContent(raw)
	for _, key := range []string{"MONGODB_URI", "MONGO_URI"} {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return mongoURIFromSecretParts(values)
}

func parseEnvContent(raw string) map[string]string {
	values := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" {
			values[key] = value
			values[strings.ToLower(key)] = value
		}
	}
	return values
}

func mongoURIFromSecretParts(values map[string]string) string {
	hosts := strings.TrimSpace(firstValue(values, "hosts", "host", "MONGODB_HOSTS", "MONGODB_HOST"))
	port := strings.TrimSpace(firstValue(values, "port", "MONGODB_PORT"))
	user := strings.TrimSpace(firstValue(values, "user", "username", "MONGODB_USER", "MONGO_INITDB_ROOT_USERNAME"))
	password := strings.TrimSpace(firstValue(values, "pass", "password", "pwd", "MONGODB_PASSWORD", "MONGO_INITDB_ROOT_PASSWORD"))
	dbName := strings.TrimSpace(firstValue(values, "db", "database", "MONGODB_DB", "DB_NAME"))
	authSource := strings.TrimSpace(firstValue(values, "authSource", "authsource", "auth_source", "MONGODB_AUTH_SOURCE"))
	if authSource == "" {
		authSource = "admin"
	}
	if hosts == "" || user == "" || password == "" || dbName == "" {
		return ""
	}

	scheme := "mongodb"
	if strings.HasPrefix(hosts, "mongodb+srv://") {
		scheme = "mongodb+srv"
		hosts = strings.TrimPrefix(hosts, "mongodb+srv://")
	} else {
		hosts = strings.TrimPrefix(hosts, "mongodb://")
	}
	hosts = strings.Trim(hosts, "/")
	if hosts == "" {
		return ""
	}
	if port != "" && !strings.Contains(hosts, ":") {
		hosts += ":" + port
	}

	uri := url.URL{
		Scheme: scheme,
		User:   url.UserPassword(user, password),
		Host:   hosts,
		Path:   "/" + dbName,
	}
	query := uri.Query()
	query.Set("authSource", authSource)
	if isTruthy(firstValue(values, "tls", "ssl", "MONGODB_TLS", "MONGODB_SSL")) {
		query.Set("tls", "true")
	}
	if caFile := strings.TrimSpace(firstValue(values, "CAFile", "cafile", "tlsCAFile", "tlscafile", "MONGODB_TLS_CA_FILE")); caFile != "" {
		query.Set("tlsCAFile", caFile)
	}
	if certificateKeyFile := strings.TrimSpace(firstValue(values, "CertificateKeyFile", "certificatekeyfile", "tlsCertificateKeyFile", "tlscertificatekeyfile", "MONGODB_TLS_CERTIFICATE_KEY_FILE")); certificateKeyFile != "" {
		query.Set("tlsCertificateKeyFile", certificateKeyFile)
	}
	uri.RawQuery = query.Encode()
	return uri.String()
}

func firstValue(values map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return ""
}

func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func databaseNameFromURI(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ""
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	name := strings.Trim(parsed.EscapedPath(), "/")
	if name == "" {
		return ""
	}
	if unescaped, err := url.PathUnescape(name); err == nil {
		name = unescaped
	}
	if strings.Contains(name, "/") {
		name = strings.Split(name, "/")[0]
	}
	return strings.TrimSpace(name)
}
