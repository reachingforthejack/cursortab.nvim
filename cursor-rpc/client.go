package cursor

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/everestmz/cursor-rpc/cursor/gen/aiserver/v1/aiserverv1connect"
)

const CursorStateDbPath = "User/globalStorage/state.vscdb"

//go:embed cursor_version.txt
var cursorVersion string

func GetCursorVersion() string {
	return strings.TrimSpace(cursorVersion)
}

type CursorCredentials struct {
	AccessToken  string `json:"authToken"`
	RefreshToken string `json:"refreshToken"`
}

func generateChecksum(machineID string) string {
	// Get current timestamp and convert to uint64
	timestamp := uint64(time.Now().UnixNano() / 1e6)

	// Convert timestamp to 6-byte array
	timestampBytes := []byte{
		byte(timestamp >> 40),
		byte(timestamp >> 32),
		byte(timestamp >> 24),
		byte(timestamp >> 16),
		byte(timestamp >> 8),
		byte(timestamp),
	}

	// Apply rolling XOR encryption (function S in the original code)
	encryptedBytes := encryptBytes(timestampBytes)

	// Convert to base64
	base64Encoded := base64.StdEncoding.EncodeToString(encryptedBytes)

	// Concatenate with machineID
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

func NewRequest[T any](credentials *CursorCredentials, message *T) *connect.Request[T] {
	req := connect.NewRequest(message)

	req.Header().Set("authorization", "bearer "+credentials.AccessToken)
	req.Header().Set("x-cursor-client-version", GetCursorVersion())
	// It doesn't look like the checksum matters. Just that we need one?
	// Either way, this is the algorithm used. I just don't know what the arg is.
	req.Header().Set("x-cursor-checksum", generateChecksum("hi"))

	return req
}

func GetBaseURL() string {
	// TODO: round robin other base URLs/do fallbacks
	return "https://api2.cursor.sh"
}

func GetRepoClientURL() string {
	// TODO: work out why this one is different
	return "https://repo42.cursor.sh"
}

func GetCursorDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir, "Library/Application Support/Cursor"), nil
	default:
		return "", fmt.Errorf("Unsure what the cursor directory is for GOOS %s - please fix if you know!", runtime.GOOS)
	}
}

func GetStateDb() (string, error) {
	cursorDir, err := GetCursorDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(cursorDir, CursorStateDbPath), nil
}

func GetDefaultCredentials() (*CursorCredentials, error) {
	stateDb, err := GetStateDb()
	if err != nil {
		return nil, err
	}

	// XXX: it's debatable whether this is good, but the alternatives are either:
	// - mattn/sqlite3, which uses CGO. I don't want to force CGO on someone, since this is a lib
	// - ncruces/go-sqlite3, which doesn't use CGO, but brings a whole wasm runtime
	// - cznic/sqlite, which uses a ton of unsafe pointers
	//
	// At the end of the day, we just need to run this once on app startup, and most local machines
	// have `sqlite3` installed. /shrug
	//
	// If the user wants to do this another way they can just provide creds via the CursorCredentials type

	var getKey = func(key string) (string, error) {
		cmd := exec.Command("sqlite3", stateDb, fmt.Sprintf(`SELECT value FROM ItemTable WHERE key = '%s';`, key))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("error getting %s (cmd %s): %w", key, cmd.String(), err)
		}
		return strings.TrimSpace(string(out)), nil
	}

	accessToken, err := getKey("cursorAuth/accessToken")
	if err != nil {
		return nil, err
	}
	refreshToken, err := getKey("cursorAuth/refreshToken")
	if err != nil {
		return nil, err
	}

	return &CursorCredentials{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func NewRepositoryServiceClient() aiserverv1connect.RepositoryServiceClient {
	return aiserverv1connect.NewRepositoryServiceClient(
		http.DefaultClient,
		GetRepoClientURL(),
	)
}

func NewAiServiceClient() aiserverv1connect.AiServiceClient {
	return aiserverv1connect.NewAiServiceClient(
		http.DefaultClient,
		GetBaseURL(),
	)
}
