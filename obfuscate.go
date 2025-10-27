// Package main contains obfuscation utilities for sensitive data in Mattermost support packets
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// ObfuscationLevel defines the security level for obfuscation
type ObfuscationLevel int

const (
	// Level3 provides maximum obfuscation - full IP and ID masking
	Level3 ObfuscationLevel = 3
)

// obfuscationCache maintains consistent mappings for obfuscated values
var obfuscationCache = make(map[string]string)

// generateConsistentHash creates a consistent hash for a given value
func generateConsistentHash(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])[:8]
}

// obfuscateIPAddress replaces IP addresses with a masked version
func obfuscateIPAddress(ip string) string {
	if cached, ok := obfuscationCache[ip]; ok {
		return cached
	}

	hash := generateConsistentHash(ip)
	obfuscated := fmt.Sprintf("XXX.XXX.XXX.%s", hash[:3])
	obfuscationCache[ip] = obfuscated
	return obfuscated
}

// obfuscateEmail replaces email addresses with masked versions
func obfuscateEmail(email string) string {
	if cached, ok := obfuscationCache[email]; ok {
		return cached
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***OBFUSCATED_EMAIL***"
	}

	userHash := generateConsistentHash(parts[0])
	domainHash := generateConsistentHash(parts[1])
	obfuscated := fmt.Sprintf("user_%s@domain_%s.com", userHash[:6], domainHash[:6])
	obfuscationCache[email] = obfuscated
	return obfuscated
}

// obfuscateURL replaces URLs with masked versions while preserving structure
func obfuscateURL(url string) string {
	if cached, ok := obfuscationCache[url]; ok {
		return cached
	}

	// Extract protocol
	protocol := "http"
	remaining := url
	if strings.HasPrefix(url, "https://") {
		protocol = "https"
		remaining = strings.TrimPrefix(url, "https://")
	} else if strings.HasPrefix(url, "http://") {
		remaining = strings.TrimPrefix(url, "http://")
	}

	// Split host and path
	parts := strings.SplitN(remaining, "/", 2)
	host := parts[0]

	// Obfuscate host (could be IP or domain)
	var obfuscatedHost string
	if regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+`).MatchString(host) {
		// It's an IP address
		ipParts := strings.Split(host, ":")
		obfuscatedHost = obfuscateIPAddress(ipParts[0])
		if len(ipParts) > 1 {
			obfuscatedHost += ":" + ipParts[1] // Keep port
		}
	} else {
		// It's a domain
		hostHash := generateConsistentHash(host)
		hostParts := strings.Split(host, ":")
		obfuscatedHost = fmt.Sprintf("host_%s.example.com", hostHash[:6])
		if len(hostParts) > 1 {
			obfuscatedHost += ":" + hostParts[1] // Keep port
		}
	}

	obfuscated := fmt.Sprintf("%s://%s", protocol, obfuscatedHost)
	if len(parts) > 1 {
		obfuscated += "/" + parts[1]
	}

	obfuscationCache[url] = obfuscated
	return obfuscated
}

// obfuscatePassword replaces passwords and secrets with a standard placeholder
func obfuscatePassword(password string) string {
	if password == "" {
		return ""
	}
	return "***REDACTED***"
}

// obfuscateAPIKey replaces API keys with a consistent hash-based placeholder
func obfuscateAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if cached, ok := obfuscationCache[key]; ok {
		return cached
	}

	hash := generateConsistentHash(key)
	obfuscated := fmt.Sprintf("OBFUSCATED_KEY_%s", hash[:8])
	obfuscationCache[key] = obfuscated
	return obfuscated
}

// obfuscateDatabaseDSN parses and obfuscates database connection strings
func obfuscateDatabaseDSN(dsn string) string {
	if dsn == "" {
		return ""
	}

	// Handle PostgreSQL DSN format: postgres://user:password@host:port/dbname?params
	postgresRegex := regexp.MustCompile(`^(postgres(?:ql)?://)([^:]+):([^@]+)@([^/]+)/([^?]+)(\?.*)?$`)
	if matches := postgresRegex.FindStringSubmatch(dsn); matches != nil {
		protocol := matches[1]
		username := matches[2]
		// matches[3] is password - we don't need to store it, just replace it
		host := matches[4]
		dbname := matches[5]
		params := matches[6]

		obfuscatedUser := fmt.Sprintf("user_%s", generateConsistentHash(username)[:6])
		obfuscatedPass := "***REDACTED***"
		obfuscatedDB := fmt.Sprintf("db_%s", generateConsistentHash(dbname)[:6])

		// Obfuscate host (could include port)
		hostParts := strings.Split(host, ":")
		obfuscatedHost := obfuscateIPAddress(hostParts[0])
		if len(hostParts) > 1 {
			obfuscatedHost += ":" + hostParts[1]
		}

		return fmt.Sprintf("%s%s:%s@%s/%s%s", protocol, obfuscatedUser, obfuscatedPass, obfuscatedHost, obfuscatedDB, params)
	}

	// Handle MySQL DSN format: user:password@tcp(host:port)/dbname?params
	mysqlRegex := regexp.MustCompile(`^([^:]+):([^@]+)@tcp\(([^)]+)\)/([^?]+)(\?.*)?$`)
	if matches := mysqlRegex.FindStringSubmatch(dsn); matches != nil {
		username := matches[1]
		// matches[2] is password - we don't need to store it, just replace it
		host := matches[3]
		dbname := matches[4]
		params := matches[5]

		obfuscatedUser := fmt.Sprintf("user_%s", generateConsistentHash(username)[:6])
		obfuscatedPass := "***REDACTED***"
		obfuscatedDB := fmt.Sprintf("db_%s", generateConsistentHash(dbname)[:6])

		// Obfuscate host
		hostParts := strings.Split(host, ":")
		obfuscatedHost := obfuscateIPAddress(hostParts[0])
		if len(hostParts) > 1 {
			obfuscatedHost += ":" + hostParts[1]
		}

		return fmt.Sprintf("%s:%s@tcp(%s)/%s%s", obfuscatedUser, obfuscatedPass, obfuscatedHost, obfuscatedDB, params)
	}

	// If we can't parse it, just redact the whole thing
	return "***REDACTED_DSN***"
}

// obfuscateUsername replaces usernames with consistent hash-based values
func obfuscateUsername(username string) string {
	if username == "" {
		return ""
	}
	if cached, ok := obfuscationCache[username]; ok {
		return cached
	}

	hash := generateConsistentHash(username)
	obfuscated := fmt.Sprintf("user_%s", hash[:8])
	obfuscationCache[username] = obfuscated
	return obfuscated
}

// ObfuscateConfigFile reads a config JSON file, obfuscates sensitive fields, and writes it back
func ObfuscateConfigFile(filepath string) error {
	DebugPrint("Obfuscating config file: " + filepath)

	// Read the file
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON into a map
	var config map[string]interface{}
	if err := json.Unmarshal(byteValue, &config); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Obfuscate sensitive fields
	obfuscateConfigData(config)

	// Write back to file
	obfuscatedJSON, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal obfuscated config: %w", err)
	}

	if err := os.WriteFile(filepath, obfuscatedJSON, 0644); err != nil {
		return fmt.Errorf("failed to write obfuscated config: %w", err)
	}

	LogMessage(infoLevel, "Config file obfuscated successfully")
	return nil
}

// obfuscateConfigData recursively obfuscates sensitive fields in config data
func obfuscateConfigData(data interface{}) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			// Check if this key contains sensitive data
			lowerKey := strings.ToLower(key)

			if strValue, ok := value.(string); ok {
				// Obfuscate based on key name
				switch {
				case strings.Contains(lowerKey, "password"):
					v[key] = obfuscatePassword(strValue)
				case strings.Contains(lowerKey, "secret"):
					v[key] = obfuscateAPIKey(strValue)
				case strings.Contains(lowerKey, "apikey") || strings.Contains(lowerKey, "api_key"):
					v[key] = obfuscateAPIKey(strValue)
				case strings.Contains(lowerKey, "token"):
					v[key] = obfuscateAPIKey(strValue)
				case strings.Contains(lowerKey, "key") && strValue != "" && len(strValue) > 10:
					v[key] = obfuscateAPIKey(strValue)
				case strings.Contains(lowerKey, "salt"):
					v[key] = obfuscateAPIKey(strValue)
				case lowerKey == "datasource" || lowerKey == "connectionurl":
					v[key] = obfuscateDatabaseDSN(strValue)
				case strings.Contains(lowerKey, "url") && (strings.HasPrefix(strValue, "http://") || strings.HasPrefix(strValue, "https://")):
					v[key] = obfuscateURL(strValue)
				case strings.Contains(lowerKey, "email") && strings.Contains(strValue, "@"):
					v[key] = obfuscateEmail(strValue)
				case strings.Contains(lowerKey, "username") && strValue != "":
					v[key] = obfuscateUsername(strValue)
				case lowerKey == "siteurl":
					v[key] = obfuscateURL(strValue)
				case strings.Contains(lowerKey, "address") || strings.Contains(lowerKey, "host"):
					// Check if it's an IP address
					if regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`).MatchString(strValue) {
						v[key] = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`).ReplaceAllStringFunc(strValue, obfuscateIPAddress)
					}
				}
			}

			// Recursively process nested structures
			obfuscateConfigData(value)
		}
	case []interface{}:
		for _, item := range v {
			obfuscateConfigData(item)
		}
	}
}

// ObfuscateLogFile reads a log file, obfuscates sensitive data, and writes it back
func ObfuscateLogFile(filepath string) error {
	DebugPrint("Obfuscating log file: " + filepath)

	// Read the file
	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	obfuscated := string(content)

	// Define regex patterns for sensitive data
	patterns := map[string]*regexp.Regexp{
		"ipv4":  regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),
		"email": regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
		"url":   regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`),
		// Token patterns - looking for long alphanumeric strings that might be tokens
		"token": regexp.MustCompile(`\b[A-Za-z0-9]{32,}\b`),
		// User IDs - looking for typical ID patterns
		"userid": regexp.MustCompile(`\b[a-z0-9]{26}\b`), // Mattermost uses 26-char IDs
	}

	// Apply obfuscation patterns
	obfuscated = patterns["ipv4"].ReplaceAllStringFunc(obfuscated, obfuscateIPAddress)
	obfuscated = patterns["email"].ReplaceAllStringFunc(obfuscated, obfuscateEmail)
	obfuscated = patterns["url"].ReplaceAllStringFunc(obfuscated, obfuscateURL)
	obfuscated = patterns["token"].ReplaceAllStringFunc(obfuscated, func(token string) string {
		// Only obfuscate if it looks like a real token (avoid false positives)
		if len(token) >= 40 {
			return obfuscateAPIKey(token)
		}
		return token
	})
	obfuscated = patterns["userid"].ReplaceAllStringFunc(obfuscated, func(id string) string {
		if cached, ok := obfuscationCache[id]; ok {
			return cached
		}
		hash := generateConsistentHash(id)
		obfuscatedID := fmt.Sprintf("id_%s", hash)
		obfuscationCache[id] = obfuscatedID
		return obfuscatedID
	})

	// Write back to file
	if err := os.WriteFile(filepath, []byte(obfuscated), 0644); err != nil {
		return fmt.Errorf("failed to write obfuscated log: %w", err)
	}

	DebugPrint("Log file obfuscated successfully")
	return nil
}

// ObfuscateDirectory processes all files in a directory for obfuscation
func ObfuscateDirectory(dir string, filePattern string) error {
	DebugPrint("Obfuscating files in directory: " + dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		filepath := dir + "/" + filename

		// Determine file type and apply appropriate obfuscation
		if strings.HasSuffix(filename, ".json") && strings.Contains(filename, "config") {
			if err := ObfuscateConfigFile(filepath); err != nil {
				LogMessage(warningLevel, "Failed to obfuscate config file "+filename+": "+err.Error())
			}
		} else if strings.HasSuffix(filename, ".log") || strings.HasSuffix(filename, ".txt") {
			if err := ObfuscateLogFile(filepath); err != nil {
				LogMessage(warningLevel, "Failed to obfuscate log file "+filename+": "+err.Error())
			}
		}
	}

	return nil
}
