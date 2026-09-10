package git

// git related types and functions

import (
	"encoding/hex"
	"net/http"
	//"github.com/labstack/echo/v5"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"
)

// statusWriter intercepts and records the written HTTP status
type StatusWriter struct {
	http.ResponseWriter
	Status int
}

type GitUpdate struct {
	OldCommit  string
	NewCommit  string
	RefName    string
	BranchName string
}

func (w *StatusWriter) WriteHeader(statusCode int) {
	w.Status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func HandleGitPush(c []byte) []GitUpdate {

	var updates []GitUpdate

	reader := bufio.NewReader(bytes.NewReader(c))

	for {
		log.Printf("uuuu")
		// Read the 4-byte hex length prefix
		lenBytes := make([]byte, 4)
		_, err := io.ReadFull(reader, lenBytes)
		if err == io.EOF {
			log.Printf("eof!!!!")
			break
		}
		if err != nil {
			//return c.String(http.StatusInternalServerError, "Error reading line length")
			return updates
		}

		// Decode hex length (e.g., "0032" -> 50 bytes)
		lineLen, err := hex.DecodeString(string(lenBytes))
		if err != nil {
			return updates
			//return c.String(http.StatusInternalServerError, "Invalid hex length")
		}

		length := int(lineLen[0])<<8 | int(lineLen[1])

		// Flush packet (0000) signals the end of the ref update section
		if length == 0 {
			break
		}

		// Read the actual data slot (length includes the 4 bytes prefix)
		dataBytes := make([]byte, length-4)
		_, err = io.ReadFull(reader, dataBytes)
		if err != nil {
			return updates
			//return c.String(http.StatusInternalServerError, "Error reading packet data")
		}

		line := string(dataBytes)
		log.Printf("uuuu, line: %s", line)

		// Parse the ref update line: "old_commit new_commit ref_name\0optional_capabilities"
		parts := strings.Split(line, " ")
		if len(parts) >= 3 {
			oldCommit := parts[0]
			newCommit := parts[1]

			// Extract ref and clean up trailing null bytes/spaces
			refName := strings.TrimSpace(strings.Split(parts[2], "\x00")[0])

			// Extract branch name if it is a heads ref
			branchName := ""
			if strings.HasPrefix(refName, "refs/heads/") {
				branchName = strings.TrimPrefix(refName, "refs/heads/")
			}

			updates = append(updates, GitUpdate{
				OldCommit:  oldCommit,
				NewCommit:  newCommit,
				RefName:    refName,
				BranchName: branchName,
			})
		}
	}

	// 3. Do something with the extracted data
	for _, update := range updates {
		fmt.Printf("Push Detected!\nBranch: %s\nOld Commit: %s\nNew Commit: %s\n",
			update.BranchName, update.OldCommit, update.NewCommit)
	}

	log.Printf("result: %s", updates)

	return updates
	// Note: To make the git client happy, you would usually forward the remaining
	// stream data to an actual `git-receive-pack` binary and return its response.
	//return c.String(http.StatusOK, "OK")
}
