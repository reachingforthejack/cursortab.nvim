package main

import (
	"connectrpc/cursor/gen/v1/aiserverv1connect"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"connectrpc.com/connect"
)

func main() {
	f, err := os.OpenFile("cursortablogs", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)

	log.Println("starting cursor tab...")

	state, err := newState()
	if err != nil {
		log.Fatalf("error creating state: %v", err)
	}

	if err := state.init(); err != nil {
		log.Fatalf("error initializing state: %v", err)
	}
}

func generateChecksum(machineID string) string {
	timestamp := uint64(time.Now().UnixNano() / 1e6)

	timestampBytes := []byte{
		byte(timestamp >> 40),
		byte(timestamp >> 32),
		byte(timestamp >> 24),
		byte(timestamp >> 16),
		byte(timestamp >> 8),
		byte(timestamp),
	}

	encryptedBytes := encryptBytes(timestampBytes)
	base64Encoded := base64.StdEncoding.EncodeToString(encryptedBytes)
	return fmt.Sprintf("%s%s", base64Encoded, machineID)
}

func encryptBytes(input []byte) []byte {
	w := byte(165)
	for i := 0; i < len(input); i++ {
		input[i] = (input[i] ^ w) + byte(i%256)
		w = input[i]
	}
	return input
}

func getAccessToken() (string, error) {
	var getKey = func(key string) (string, error) {
		homeDir := os.Getenv("HOME")
		cmd := exec.Command("sqlite3", fmt.Sprintf("%s/Library/Application Support/Cursor/User/globalStorage/state.vscdb", homeDir), fmt.Sprintf(`SELECT value FROM ItemTable WHERE key = '%s';`, key))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("error getting %s (cmd %s): %w", key, cmd.String(), err)
		}
		return strings.TrimSpace(string(out)), nil
	}

	accessToken, err := getKey("cursorAuth/accessToken")
	if err != nil {
		return "", err
	}

	return accessToken, nil
}

func newAiServiceClient() aiserverv1connect.AiServiceClient {
	return aiserverv1connect.NewAiServiceClient(
		http.DefaultClient,
		"https://api2.cursor.sh",
	)
}

func newRequest[T any](accessToken, checksum string, message *T) *connect.Request[T] {
	req := connect.NewRequest(message)

	req.Header().Set("authorization", "bearer "+accessToken)
	req.Header().Set("x-cursor-client-version", "0.45.0")
	req.Header().Set("x-cursor-checksum", checksum)

	return req
}
