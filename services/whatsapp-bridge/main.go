package main

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	_ "github.com/lib/pq"
	"github.com/mdp/qrterminal"

	"bytes"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Project configurations
var PROJECTS = map[string]map[string]string{
	"Lawley": {
		"group_jid":          "120363418298130331@g.us",
		"project_name":       "Lawley",
		"group_description": "Lawley Activation 3 group",
	},
	"Velo Test": {
		"group_jid":          "120363421664266245@g.us",
		"project_name":       "Velo Test",
		"group_description": "Velo Test group",
	},
	"Mohadin": {
		"group_jid":          "120363421532174586@g.us",
		"project_name":       "Mohadin",
		"group_description": "Mohadin Activations group",
	},
}

// Neon database configuration
const NEON_DB_URL = "postgresql://neondb_owner:npg_RIgDxzo4St6d@ep-damp-credit-a857vku0-pooler.eastus2.azure.neon.tech/neondb?sslmode=require&channel_binding=require"

// Google Sheets configuration
const GOOGLE_SHEETS_ID = "1TYxDLyCqDHr0Imb5j7X4uJhxccgJTO0KrDVAD0Ja0Dk"
const GOOGLE_CREDENTIALS_PATH = "./credentials.json"

// Project-specific Google Sheets tab mapping
var PROJECT_SHEETS_TABS = map[string]string{
	"Velo Test": "Velo Test",                  // Live production - direct writes
	"Mohadin":   "Mohadin WA_Tool Monitor",    // Safe monitor tab
	"Lawley":    "Lawley WA_Tool Monitor",     // Safe monitor tab
}

// Drop number pattern
var dropPattern = regexp.MustCompile(`DR\d+`)

// Get project name from JID
func getProjectNameByJID(jid string) string {
	// Look up the JID in the projects map
	for _, config := range PROJECTS {
		if config["group_jid"] == jid {
			return config["project_name"]
		}
	}
	
	// Return empty string if no match found (don't store non-project chats)
	return ""
}

// Message represents a chat message for our client
type Message struct {
	Time      time.Time
	Sender    string
	Content   string
	IsFromMe  bool
	MediaType string
	Filename  string
}

// Database handler for storing message history
type MessageStore struct {
	db *sql.DB
}

// Initialize message store
func NewMessageStore() (*MessageStore, error) {
	// Create directory for database if it doesn't exist
	if err := os.MkdirAll("store", 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %v", err)
	}

	// Open SQLite database for messages
	db, err := sql.Open("sqlite3", "file:store/messages.db?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open message database: %v", err)
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS chats (
			jid TEXT PRIMARY KEY,
			name TEXT,
			last_message_time TIMESTAMP,
			project_name TEXT
		);
		
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT,
			chat_jid TEXT,
			sender TEXT,
			content TEXT,
			timestamp TIMESTAMP,
			is_from_me BOOLEAN,
			media_type TEXT,
			filename TEXT,
			url TEXT,
			media_key BLOB,
			file_sha256 BLOB,
			file_enc_sha256 BLOB,
			file_length INTEGER,
			PRIMARY KEY (id, chat_jid),
			FOREIGN KEY (chat_jid) REFERENCES chats(jid)
		);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return &MessageStore{db: db}, nil
}

// Close the database connection
func (store *MessageStore) Close() error {
	return store.db.Close()
}

// Store a chat in the database
func (store *MessageStore) StoreChat(jid, name string, lastMessageTime time.Time) error {
	// Determine project name based on JID
	projectName := getProjectNameByJID(jid)

	fmt.Printf("💾 Storing chat: JID=%s, Name=%s, Project=%s\n", jid, name, projectName)

	// Store all chats, but only set project_name for tracked projects
	// This ensures foreign key constraints work for all messages
	result, err := store.db.Exec(
		"INSERT OR REPLACE INTO chats (jid, name, last_message_time, project_name) VALUES (?, ?, ?, ?)",
		jid, name, lastMessageTime, projectName,
	)

	if err != nil {
		fmt.Printf("❌ FAILED to store chat: %v\n", err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("✅ Chat stored successfully: %d rows affected\n", rowsAffected)
	return nil
}

// Store a message in the database
func (store *MessageStore) StoreMessage(id, chatJID, sender, content string, timestamp time.Time, isFromMe bool,
	mediaType, filename, url string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength uint64) error {
	// Only store if there's actual content or media
	if content == "" && mediaType == "" {
		return nil
	}

	_, err := store.db.Exec(
		`INSERT OR REPLACE INTO messages 
		(id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, chatJID, sender, content, timestamp, isFromMe, mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength,
	)
	return err
}

// Get messages from a chat
func (store *MessageStore) GetMessages(chatJID string, limit int) ([]Message, error) {
	rows, err := store.db.Query(
		"SELECT sender, content, timestamp, is_from_me, media_type, filename FROM messages WHERE chat_jid = ? ORDER BY timestamp DESC LIMIT ?",
		chatJID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var timestamp time.Time
		err := rows.Scan(&msg.Sender, &msg.Content, &timestamp, &msg.IsFromMe, &msg.MediaType, &msg.Filename)
		if err != nil {
			return nil, err
		}
		msg.Time = timestamp
		messages = append(messages, msg)
	}

	return messages, nil
}

// Get all chats
func (store *MessageStore) GetChats() (map[string]time.Time, error) {
	rows, err := store.db.Query("SELECT jid, last_message_time FROM chats ORDER BY last_message_time DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chats := make(map[string]time.Time)
	for rows.Next() {
		var jid string
		var lastMessageTime time.Time
		err := rows.Scan(&jid, &lastMessageTime)
		if err != nil {
			return nil, err
		}
		chats[jid] = lastMessageTime
	}

	return chats, nil
}

// Extract text content from a message
func extractTextContent(msg *waProto.Message) string {
	if msg == nil {
		return ""
	}

	// Try to get text content
	if text := msg.GetConversation(); text != "" {
		return text
	} else if extendedText := msg.GetExtendedTextMessage(); extendedText != nil {
		return extendedText.GetText()
	}

	// For now, we're ignoring non-text messages
	return ""
}

// SendMessageResponse represents the response for the send message API
type SendMessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SendMessageRequest represents the request body for the send message API
type SendMessageRequest struct {
	Recipient string `json:"recipient"`
	Message   string `json:"message"`
	MediaPath string `json:"media_path,omitempty"`
}

// Function to send a WhatsApp message
func sendWhatsAppMessage(client *whatsmeow.Client, recipient string, message string, mediaPath string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	// Create JID for recipient
	var recipientJID types.JID
	var err error

	// Check if recipient is a JID
	isJID := strings.Contains(recipient, "@")

	if isJID {
		// Parse the JID string
		recipientJID, err = types.ParseJID(recipient)
		if err != nil {
			return false, fmt.Sprintf("Error parsing JID: %v", err)
		}
	} else {
		// Create JID from phone number
		recipientJID = types.JID{
			User:   recipient,
			Server: "s.whatsapp.net", // For personal chats
		}
	}

	msg := &waProto.Message{}

	// Check if we have media to send
	if mediaPath != "" {
		// Read media file
		mediaData, err := os.ReadFile(mediaPath)
		if err != nil {
			return false, fmt.Sprintf("Error reading media file: %v", err)
		}

		// Determine media type and mime type based on file extension
		fileExt := strings.ToLower(mediaPath[strings.LastIndex(mediaPath, ".")+1:])
		var mediaType whatsmeow.MediaType
		var mimeType string

		// Handle different media types
		switch fileExt {
		// Image types
		case "jpg", "jpeg":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/jpeg"
		case "png":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/png"
		case "gif":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/gif"
		case "webp":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/webp"

		// Audio types
		case "ogg":
			mediaType = whatsmeow.MediaAudio
			mimeType = "audio/ogg; codecs=opus"

		// Video types
		case "mp4":
			mediaType = whatsmeow.MediaVideo
			mimeType = "video/mp4"
		case "avi":
			mediaType = whatsmeow.MediaVideo
			mimeType = "video/avi"
		case "mov":
			mediaType = whatsmeow.MediaVideo
			mimeType = "video/quicktime"

		// Document types (for any other file type)
		default:
			mediaType = whatsmeow.MediaDocument
			mimeType = "application/octet-stream"
		}

		// Upload media to WhatsApp servers
		resp, err := client.Upload(context.Background(), mediaData, mediaType)
		if err != nil {
			return false, fmt.Sprintf("Error uploading media: %v", err)
		}

		fmt.Println("Media uploaded", resp)

		// Create the appropriate message type based on media type
		switch mediaType {
		case whatsmeow.MediaImage:
			msg.ImageMessage = &waProto.ImageMessage{
				Caption:       proto.String(message),
				Mimetype:      proto.String(mimeType),
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &resp.FileLength,
			}
		case whatsmeow.MediaAudio:
			// Handle ogg audio files
			var seconds uint32 = 30 // Default fallback
			var waveform []byte = nil

			// Try to analyze the ogg file
			if strings.Contains(mimeType, "ogg") {
				analyzedSeconds, analyzedWaveform, err := analyzeOggOpus(mediaData)
				if err == nil {
					seconds = analyzedSeconds
					waveform = analyzedWaveform
				} else {
					return false, fmt.Sprintf("Failed to analyze Ogg Opus file: %v", err)
				}
			} else {
				fmt.Printf("Not an Ogg Opus file: %s\n", mimeType)
			}

			msg.AudioMessage = &waProto.AudioMessage{
				Mimetype:      proto.String(mimeType),
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &resp.FileLength,
				Seconds:       proto.Uint32(seconds),
				PTT:           proto.Bool(true),
				Waveform:      waveform,
			}
		case whatsmeow.MediaVideo:
			msg.VideoMessage = &waProto.VideoMessage{
				Caption:       proto.String(message),
				Mimetype:      proto.String(mimeType),
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &resp.FileLength,
			}
		case whatsmeow.MediaDocument:
			msg.DocumentMessage = &waProto.DocumentMessage{
				Title:         proto.String(mediaPath[strings.LastIndex(mediaPath, "/")+1:]),
				Caption:       proto.String(message),
				Mimetype:      proto.String(mimeType),
				URL:           &resp.URL,
				DirectPath:    &resp.DirectPath,
				MediaKey:      resp.MediaKey,
				FileEncSHA256: resp.FileEncSHA256,
				FileSHA256:    resp.FileSHA256,
				FileLength:    &resp.FileLength,
			}
		}
	} else {
		msg.Conversation = proto.String(message)
	}

	// Send message
	_, err = client.SendMessage(context.Background(), recipientJID, msg)

	if err != nil {
		return false, fmt.Sprintf("Error sending message: %v", err)
	}

	return true, fmt.Sprintf("Message sent to %s", recipient)
}

// Extract media info from a message
func extractMediaInfo(msg *waProto.Message) (mediaType string, filename string, url string, mediaKey []byte, fileSHA256 []byte, fileEncSHA256 []byte, fileLength uint64) {
	if msg == nil {
		return "", "", "", nil, nil, nil, 0
	}

	// Check for image message
	if img := msg.GetImageMessage(); img != nil {
		return "image", "image_" + time.Now().Format("20060102_150405") + ".jpg",
			img.GetURL(), img.GetMediaKey(), img.GetFileSHA256(), img.GetFileEncSHA256(), img.GetFileLength()
	}

	// Check for video message
	if vid := msg.GetVideoMessage(); vid != nil {
		return "video", "video_" + time.Now().Format("20060102_150405") + ".mp4",
			vid.GetURL(), vid.GetMediaKey(), vid.GetFileSHA256(), vid.GetFileEncSHA256(), vid.GetFileLength()
	}

	// Check for audio message
	if aud := msg.GetAudioMessage(); aud != nil {
		return "audio", "audio_" + time.Now().Format("20060102_150405") + ".ogg",
			aud.GetURL(), aud.GetMediaKey(), aud.GetFileSHA256(), aud.GetFileEncSHA256(), aud.GetFileLength()
	}

	// Check for document message
	if doc := msg.GetDocumentMessage(); doc != nil {
		filename := doc.GetFileName()
		if filename == "" {
			filename = "document_" + time.Now().Format("20060102_150405")
		}
		return "document", filename,
			doc.GetURL(), doc.GetMediaKey(), doc.GetFileSHA256(), doc.GetFileEncSHA256(), doc.GetFileLength()
	}

	return "", "", "", nil, nil, nil, 0
}

// Handle regular incoming messages with media support
func handleMessage(client *whatsmeow.Client, messageStore *MessageStore, msg *events.Message, logger waLog.Logger) {
	// VELO TEST DEPLOYMENT: Only process messages from Velo Test group
	chatJID := msg.Info.Chat.String()
	veloTestJID := "120363421664266245@g.us"
	
	if chatJID != veloTestJID {
		// Silently ignore messages from other chats/groups for privacy
		return
	}
	
	// Log immediate debug info
	fmt.Printf("🎯 handleMessage called! Chat: %s, Sender: %s, IsFromMe: %v\n",
		msg.Info.Chat.String(), msg.Info.Sender.String(), msg.Info.IsFromMe)
	logger.Infof("🎯 handleMessage called! Chat: %s, Sender: %s, IsFromMe: %v",
		msg.Info.Chat.String(), msg.Info.Sender.String(), msg.Info.IsFromMe)

	// Save message to database  
	sender := msg.Info.Sender.User

	// Get appropriate chat name (pass nil for conversation since we don't have one for regular messages)
	fmt.Printf("🔍 Getting chat name for JID: %s\n", chatJID)
	name := GetChatName(client, messageStore, msg.Info.Chat, chatJID, nil, sender, logger)
	fmt.Printf("✅ Chat name retrieved: %s\n", name)

	// Update chat in database with the message timestamp (keeps last message time updated)
	err := messageStore.StoreChat(chatJID, name, msg.Info.Timestamp)
	if err != nil {
		logger.Warnf("Failed to store chat: %v", err)
	}

	// Extract text content
	content := extractTextContent(msg.Message)

	// Extract media info
	mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := extractMediaInfo(msg.Message)

	// Skip if there's no content and no media
	if content == "" && mediaType == "" {
		return
	}

	// Store message in database
	fmt.Printf("💾 Storing message in SQLite: ID=%s, Chat=%s, Content='%s'\n",
		msg.Info.ID, chatJID, content)
	err = messageStore.StoreMessage(
		msg.Info.ID,
		chatJID,
		sender,
		content,
		msg.Info.Timestamp,
		msg.Info.IsFromMe,
		mediaType,
		filename,
		url,
		mediaKey,
		fileSHA256,
		fileEncSHA256,
		fileLength,
	)

	if err != nil {
		fmt.Printf("❌ FAILED to store message: %v\n", err)
		logger.Warnf("Failed to store message: %v", err)
	} else {
		fmt.Printf("✅ SUCCESS: Message stored in SQLite\n")
		// Log message reception
		timestamp := msg.Info.Timestamp.Format("2006-01-02 15:04:05")
		direction := "←"
		if msg.Info.IsFromMe {
			direction = "→"
		}

		// Log based on message type
		if mediaType != "" {
			fmt.Printf("[%s] %s %s: [%s: %s] %s\n", timestamp, direction, sender, mediaType, filename, content)
		} else if content != "" {
			fmt.Printf("[%s] %s %s: %s\n", timestamp, direction, sender, content)
		}

		// Process drop numbers if content exists
		// TEMPORARILY REMOVED IsFromMe restriction to fix processing issue
		if content != "" {
			fmt.Printf("🎯 Processing drop numbers from message: '%s' (IsFromMe: %v)\n", content, msg.Info.IsFromMe)
			processDropNumbers(content, chatJID, sender, msg.Info.Timestamp, logger)
		}
	}
}

// DownloadMediaRequest represents the request body for the download media API
type DownloadMediaRequest struct {
	MessageID string `json:"message_id"`
	ChatJID   string `json:"chat_jid"`
}

// DownloadMediaResponse represents the response for the download media API
type DownloadMediaResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path,omitempty"`
}

// Store additional media info in the database
func (store *MessageStore) StoreMediaInfo(id, chatJID, url string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength uint64) error {
	_, err := store.db.Exec(
		"UPDATE messages SET url = ?, media_key = ?, file_sha256 = ?, file_enc_sha256 = ?, file_length = ? WHERE id = ? AND chat_jid = ?",
		url, mediaKey, fileSHA256, fileEncSHA256, fileLength, id, chatJID,
	)
	return err
}

// Get media info from the database
func (store *MessageStore) GetMediaInfo(id, chatJID string) (string, string, string, []byte, []byte, []byte, uint64, error) {
	var mediaType, filename, url string
	var mediaKey, fileSHA256, fileEncSHA256 []byte
	var fileLength uint64

	err := store.db.QueryRow(
		"SELECT media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length FROM messages WHERE id = ? AND chat_jid = ?",
		id, chatJID,
	).Scan(&mediaType, &filename, &url, &mediaKey, &fileSHA256, &fileEncSHA256, &fileLength)

	return mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength, err
}

// MediaDownloader implements the whatsmeow.DownloadableMessage interface
type MediaDownloader struct {
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileLength    uint64
	FileSHA256    []byte
	FileEncSHA256 []byte
	MediaType     whatsmeow.MediaType
}

// GetDirectPath implements the DownloadableMessage interface
func (d *MediaDownloader) GetDirectPath() string {
	return d.DirectPath
}

// GetURL implements the DownloadableMessage interface
func (d *MediaDownloader) GetURL() string {
	return d.URL
}

// GetMediaKey implements the DownloadableMessage interface
func (d *MediaDownloader) GetMediaKey() []byte {
	return d.MediaKey
}

// GetFileLength implements the DownloadableMessage interface
func (d *MediaDownloader) GetFileLength() uint64 {
	return d.FileLength
}

// GetFileSHA256 implements the DownloadableMessage interface
func (d *MediaDownloader) GetFileSHA256() []byte {
	return d.FileSHA256
}

// GetFileEncSHA256 implements the DownloadableMessage interface
func (d *MediaDownloader) GetFileEncSHA256() []byte {
	return d.FileEncSHA256
}

// GetMediaType implements the DownloadableMessage interface
func (d *MediaDownloader) GetMediaType() whatsmeow.MediaType {
	return d.MediaType
}

// Function to download media from a message
func downloadMedia(client *whatsmeow.Client, messageStore *MessageStore, messageID, chatJID string) (bool, string, string, string, error) {
	// Query the database for the message
	var mediaType, filename, url string
	var mediaKey, fileSHA256, fileEncSHA256 []byte
	var fileLength uint64
	var err error

	// First, check if we already have this file
	chatDir := fmt.Sprintf("store/%s", strings.ReplaceAll(chatJID, ":", "_"))
	localPath := ""

	// Get media info from the database
	mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength, err = messageStore.GetMediaInfo(messageID, chatJID)

	if err != nil {
		// Try to get basic info if extended info isn't available
		err = messageStore.db.QueryRow(
			"SELECT media_type, filename FROM messages WHERE id = ? AND chat_jid = ?",
			messageID, chatJID,
		).Scan(&mediaType, &filename)

		if err != nil {
			return false, "", "", "", fmt.Errorf("failed to find message: %v", err)
		}
	}

	// Check if this is a media message
	if mediaType == "" {
		return false, "", "", "", fmt.Errorf("not a media message")
	}

	// Create directory for the chat if it doesn't exist
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return false, "", "", "", fmt.Errorf("failed to create chat directory: %v", err)
	}

	// Generate a local path for the file
	localPath = fmt.Sprintf("%s/%s", chatDir, filename)

	// Get absolute path
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return false, "", "", "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Check if file already exists
	if _, err := os.Stat(localPath); err == nil {
		// File exists, return it
		return true, mediaType, filename, absPath, nil
	}

	// If we don't have all the media info we need, we can't download
	if url == "" || len(mediaKey) == 0 || len(fileSHA256) == 0 || len(fileEncSHA256) == 0 || fileLength == 0 {
		return false, "", "", "", fmt.Errorf("incomplete media information for download")
	}

	fmt.Printf("Attempting to download media for message %s in chat %s...\n", messageID, chatJID)

	// Extract direct path from URL
	directPath := extractDirectPathFromURL(url)

	// Create a downloader that implements DownloadableMessage
	var waMediaType whatsmeow.MediaType
	switch mediaType {
	case "image":
		waMediaType = whatsmeow.MediaImage
	case "video":
		waMediaType = whatsmeow.MediaVideo
	case "audio":
		waMediaType = whatsmeow.MediaAudio
	case "document":
		waMediaType = whatsmeow.MediaDocument
	default:
		return false, "", "", "", fmt.Errorf("unsupported media type: %s", mediaType)
	}

	downloader := &MediaDownloader{
		URL:           url,
		DirectPath:    directPath,
		MediaKey:      mediaKey,
		FileLength:    fileLength,
		FileSHA256:    fileSHA256,
		FileEncSHA256: fileEncSHA256,
		MediaType:     waMediaType,
	}

	// Download the media using whatsmeow client
	mediaData, err := client.Download(context.Background(), downloader)
	if err != nil {
		return false, "", "", "", fmt.Errorf("failed to download media: %v", err)
	}

	// Save the downloaded media to file
	if err := os.WriteFile(localPath, mediaData, 0644); err != nil {
		return false, "", "", "", fmt.Errorf("failed to save media file: %v", err)
	}

	fmt.Printf("Successfully downloaded %s media to %s (%d bytes)\n", mediaType, absPath, len(mediaData))
	return true, mediaType, filename, absPath, nil
}

// Extract direct path from a WhatsApp media URL
func extractDirectPathFromURL(url string) string {
	// The direct path is typically in the URL, we need to extract it
	// Example URL: https://mmg.whatsapp.net/v/t62.7118-24/13812002_698058036224062_3424455886509161511_n.enc?ccb=11-4&oh=...

	// Find the path part after the domain
	parts := strings.SplitN(url, ".net/", 2)
	if len(parts) < 2 {
		return url // Return original URL if parsing fails
	}

	pathPart := parts[1]

	// Remove query parameters
	pathPart = strings.SplitN(pathPart, "?", 2)[0]

	// Create proper direct path format
	return "/" + pathPart
}

// Start a REST API server to expose the WhatsApp client functionality
func startRESTServer(client *whatsmeow.Client, messageStore *MessageStore, port int) {
	// Handler for sending messages
	http.HandleFunc("/api/send", func(w http.ResponseWriter, r *http.Request) {
		// Only allow POST requests
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the request body
		var req SendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Validate request
		if req.Recipient == "" {
			http.Error(w, "Recipient is required", http.StatusBadRequest)
			return
		}

		if req.Message == "" && req.MediaPath == "" {
			http.Error(w, "Message or media path is required", http.StatusBadRequest)
			return
		}

		fmt.Println("Received request to send message", req.Message, req.MediaPath)

		// Send the message
		success, message := sendWhatsAppMessage(client, req.Recipient, req.Message, req.MediaPath)
		fmt.Println("Message sent", success, message)
		// Set response headers
		w.Header().Set("Content-Type", "application/json")

		// Set appropriate status code
		if !success {
			w.WriteHeader(http.StatusInternalServerError)
		}

		// Send response
		json.NewEncoder(w).Encode(SendMessageResponse{
			Success: success,
			Message: message,
		})
	})

	// Handler for downloading media
	http.HandleFunc("/api/download", func(w http.ResponseWriter, r *http.Request) {
		// Only allow POST requests
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the request body
		var req DownloadMediaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Validate request
		if req.MessageID == "" || req.ChatJID == "" {
			http.Error(w, "Message ID and Chat JID are required", http.StatusBadRequest)
			return
		}

		// Download the media
		success, mediaType, filename, path, err := downloadMedia(client, messageStore, req.MessageID, req.ChatJID)

		// Set response headers
		w.Header().Set("Content-Type", "application/json")

		// Handle download result
		if !success || err != nil {
			errMsg := "Unknown error"
			if err != nil {
				errMsg = err.Error()
			}

			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(DownloadMediaResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to download media: %s", errMsg),
			})
			return
		}

		// Send successful response
		json.NewEncoder(w).Encode(DownloadMediaResponse{
			Success:  true,
			Message:  fmt.Sprintf("Successfully downloaded %s media", mediaType),
			Filename: filename,
			Path:     path,
		})
	})

	// Start the server
	serverAddr := fmt.Sprintf(":%d", port)
	fmt.Printf("Starting REST API server on %s...\n", serverAddr)

	// Run server in a goroutine so it doesn't block
	go func() {
		if err := http.ListenAndServe(serverAddr, nil); err != nil {
			fmt.Printf("REST API server error: %v\n", err)
		}
	}()
}

func main() {
	// Set up logger - reduced logging to prevent rate limiting
	logger := waLog.Stdout("Client", "WARN", true)
	logger.Warnf("Starting WhatsApp client...")

	// Create database connection for storing session data
	dbLog := waLog.Stdout("Database", "WARN", true)

	// Create directory for database if it doesn't exist
	if err := os.MkdirAll("store", 0755); err != nil {
		logger.Errorf("Failed to create store directory: %v", err)
		return
	}

	container, err := sqlstore.New(context.Background(), "sqlite3", "file:store/whatsapp.db?_foreign_keys=on", dbLog)
	if err != nil {
		logger.Errorf("Failed to connect to database: %v", err)
		return
	}

	// Get device store - This contains session information
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		if err == sql.ErrNoRows {
			// No device exists, create one
			deviceStore = container.NewDevice()
			logger.Infof("Created new device")
		} else {
			logger.Errorf("Failed to get device: %v", err)
			return
		}
	}

	// Create client instance
	client := whatsmeow.NewClient(deviceStore, logger)
	if client == nil {
		logger.Errorf("Failed to create WhatsApp client")
		return
	}

	// Initialize message store
	messageStore, err := NewMessageStore()
	if err != nil {
		logger.Errorf("Failed to initialize message store: %v", err)
		return
	}
	defer messageStore.Close()

	// Setup event handling for messages and history sync
	client.AddEventHandler(func(evt interface{}) {
		fmt.Printf("🚀 EVENT RECEIVED: %T\n", evt)
		switch v := evt.(type) {
		case *events.Message:
			// Process regular messages
			fmt.Printf("🔥 Message event received: ID=%s, Chat=%s, Sender=%s\n",
				v.Info.ID, v.Info.Chat.String(), v.Info.Sender.String())
			logger.Infof("🔥 Message event received: ID=%s, Chat=%s, Sender=%s",
				v.Info.ID, v.Info.Chat.String(), v.Info.Sender.String())
			handleMessage(client, messageStore, v, logger)

		case *events.HistorySync:
			// Process history sync events
			handleHistorySync(client, messageStore, v, logger)

		case *events.Connected:
			logger.Infof("Connected to WhatsApp")

	case *events.LoggedOut:
		logger.Warnf("Device logged out, please scan QR code to log in again")

	case *events.Receipt:
		// Handle receipt events - these indicate message status changes
		handleReceiptEvent(client, messageStore, v, logger)
	
	default:
		// Log unknown events for debugging
		logger.Infof("Received unhandled event type: %T", v)
		}
	})

	// Connect to WhatsApp
	err = client.Connect()
	if err != nil {
		logger.Errorf("Failed to connect: %v", err)
		return
	}

	// Handle pairing if necessary
	if client.Store.ID == nil {
		// No ID stored, this is a new client, need to pair with phone
		phoneNumber := os.Getenv("WHATSAPP_PHONE_NUMBER")
		if phoneNumber == "" {
			phoneNumber = "+27640412391" // Default to Louis's number
		}

		fmt.Printf("📱 Using phone number pairing for: %s\n", phoneNumber)
		logger.Infof("Starting phone number pairing for: %s", phoneNumber)

		code, err := client.PairPhone(context.Background(), phoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
		if err != nil {
			logger.Errorf("Failed to request pairing code, falling back to QR: %v", err)
			// Fallback to QR code if phone pairing fails
			qrChan, _ := client.GetQRChannel(context.Background())
			for evt := range qrChan {
				if evt.Event == "code" {
					fmt.Println("\nScan this QR code with your WhatsApp app:")
					qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				} else {
					logger.Infof("QR channel event: %s", evt.Event)
				}
			}
		} else {
			fmt.Printf("\n🔑 PAIRING CODE: %s\n\n", code)
			fmt.Println("📱 Enter this code in WhatsApp on your phone")
		}
	}

	// Wait a moment for connection to stabilize
	time.Sleep(2 * time.Second)

	if !client.IsConnected() {
		logger.Errorf("Failed to establish stable connection")
		return
	}

	fmt.Println("\n✓ Connected to WhatsApp! Type 'help' for commands.")

	// Start REST API server
	startRESTServer(client, messageStore, 8080)

	// Create a channel to keep the main goroutine alive
	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("REST server is running. Press Ctrl+C to disconnect and exit.")

	// Wait for termination signal
	<-exitChan

	fmt.Println("Disconnecting...")
	// Disconnect client
	client.Disconnect()
}

// GetChatName determines the appropriate name for a chat based on JID and other info
func GetChatName(client *whatsmeow.Client, messageStore *MessageStore, jid types.JID, chatJID string, conversation interface{}, sender string, logger waLog.Logger) string {
	// First, check if chat already exists in database with a name
	var existingName string
	err := messageStore.db.QueryRow("SELECT name FROM chats WHERE jid = ?", chatJID).Scan(&existingName)
	if err == nil && existingName != "" {
		// Chat exists with a name, use that
		logger.Infof("Using existing chat name for %s: %s", chatJID, existingName)
		return existingName
	}

	// Need to determine chat name
	var name string

	if jid.Server == "g.us" {
		// This is a group chat
		logger.Infof("Getting name for group: %s", chatJID)

		// Use conversation data if provided (from history sync)
		if conversation != nil {
			// Extract name from conversation if available
			// This uses type assertions to handle different possible types
			var displayName, convName *string
			// Try to extract the fields we care about regardless of the exact type
			v := reflect.ValueOf(conversation)
			if v.Kind() == reflect.Ptr && !v.IsNil() {
				v = v.Elem()

				// Try to find DisplayName field
				if displayNameField := v.FieldByName("DisplayName"); displayNameField.IsValid() && displayNameField.Kind() == reflect.Ptr && !displayNameField.IsNil() {
					dn := displayNameField.Elem().String()
					displayName = &dn
				}

				// Try to find Name field
				if nameField := v.FieldByName("Name"); nameField.IsValid() && nameField.Kind() == reflect.Ptr && !nameField.IsNil() {
					n := nameField.Elem().String()
					convName = &n
				}
			}

			// Use the name we found
			if displayName != nil && *displayName != "" {
				name = *displayName
			} else if convName != nil && *convName != "" {
				name = *convName
			}
		}

		// If we didn't get a name, try group info
		if name == "" {
			groupInfo, err := client.GetGroupInfo(jid)
			if err == nil && groupInfo.Name != "" {
				name = groupInfo.Name
			} else {
				// Fallback name for groups
				name = fmt.Sprintf("Group %s", jid.User)
			}
		}

		logger.Infof("Using group name: %s", name)
	} else {
		// This is an individual contact
		logger.Infof("Getting name for contact: %s", chatJID)

		// Just use contact info (full name)
		contact, err := client.Store.Contacts.GetContact(context.Background(), jid)
		if err == nil && contact.FullName != "" {
			name = contact.FullName
		} else if sender != "" {
			// Fallback to sender
			name = sender
		} else {
			// Last fallback to JID
			name = jid.User
		}

		logger.Infof("Using contact name: %s", name)
	}

	return name
}

// Create or update QA photo review record in Neon database
func createQAPhotoReview(dropNumber, projectName, userName string, reviewDate time.Time) error {
	db, err := sql.Open("postgres", NEON_DB_URL)
	if err != nil {
		return fmt.Errorf("failed to connect to Neon database: %v", err)
	}
	defer db.Close()

	// Check if QA review already exists for this drop and date
	var existingID string
	err = db.QueryRow("SELECT id FROM qa_photo_reviews WHERE drop_number = $1 AND review_date = $2", 
		dropNumber, reviewDate.Format("2006-01-02")).Scan(&existingID)
	
	if err == nil {
		// Record exists - this is a resubmission, update it
		fmt.Printf("🔄 Drop %s already has QA review for %s - updating as resubmission\n", 
			dropNumber, reviewDate.Format("2006-01-02"))
		
		// Reset completion status and add resubmission note
		_, updateErr := db.Exec(`
			UPDATE qa_photo_reviews 
			SET 
				incomplete = FALSE,
				feedback_sent = NULL,
				comment = COALESCE(comment, '') || $1,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = $2
		`, fmt.Sprintf("\n--- RESUBMITTED %s ---\nPhotos updated by %s. QA can continue review.\n", 
			time.Now().Format("2006-01-02 15:04:05"), userName), existingID)
		
		if updateErr != nil {
			return fmt.Errorf("failed to update QA photo review for resubmission: %v", updateErr)
		}
		
		fmt.Printf("✅ Updated QA photo review for resubmission: %s\n", dropNumber)
		return nil
	}
	
	// Record doesn't exist - create new one
	if err == sql.ErrNoRows {
		_, err = db.Exec(`
			INSERT INTO qa_photo_reviews (
				drop_number, review_date, user_name, project,
				step_01_property_frontage, step_02_location_before_install,
				step_03_outside_cable_span, step_04_home_entry_outside,
				step_05_home_entry_inside, step_06_fibre_entry_to_ont,
				step_07_patched_labelled_drop, step_08_work_area_completion,
				step_09_ont_barcode_scan, step_10_ups_serial_number,
				step_11_powermeter_reading, step_12_powermeter_at_ont,
				step_13_active_broadband_light, step_14_customer_signature,
				outstanding_photos_loaded_to_1map, comment
			) VALUES (
				$1, $2, $3, $4,
				FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE,
				FALSE, FALSE, FALSE, FALSE, FALSE, FALSE, FALSE,
				$5
			)
		`, dropNumber, reviewDate.Format("2006-01-02"), userName, projectName,
			fmt.Sprintf("Auto-created from WhatsApp on %s", time.Now().Format("2006-01-02 15:04:05")))

		if err != nil {
			return fmt.Errorf("failed to create QA photo review: %v", err)
		}

		fmt.Printf("✅ Created QA photo review for %s (user: %s, project: %s)\n", dropNumber, userName, projectName)
		return nil
	}
	
	// Some other database error
	return fmt.Errorf("failed to check existing QA review: %v", err)
}

// Find first empty row starting from row 17
func findFirstEmptyRow(srv *sheets.Service, tabName string, ctx context.Context) (int, error) {
	// Start checking from row 17
	startRow := 17

	// Read rows 17-100 to find first empty one
	readRange := fmt.Sprintf("%s!A%d:A%d", tabName, startRow, startRow + 83) // Check rows 17-100
	resp, err := srv.Spreadsheets.Values.Get(GOOGLE_SHEETS_ID, readRange).Context(ctx).Do()
	if err != nil {
		return startRow, fmt.Errorf("failed to read rows to find empty spot: %v", err)
	}

	// If no data found, start at row 17
	if len(resp.Values) == 0 {
		return startRow, nil
	}

	// Find first row where Column A is empty
	for i, row := range resp.Values {
		if len(row) == 0 || row[0] == nil || row[0] == "" {
			return startRow + i, nil
		}
	}

	// If we reach here, all returned rows have data
	// The next empty row is startRow + len(resp.Values)
	nextEmptyRow := startRow + len(resp.Values)
	
	// Make sure we don't exceed reasonable bounds
	if nextEmptyRow > 200 {
		fmt.Printf("⚠️  Warning: Next empty row is %d, which seems very high. Using row 101 instead.\n", nextEmptyRow)
		return 101, nil
	}
	
	fmt.Printf("📍 Next empty row determined: %d (after %d filled rows)\n", nextEmptyRow, len(resp.Values))
	return nextEmptyRow, nil
}

// Write drop number to Google Sheets
func writeToGoogleSheets(dropNumber, projectName, userName string, reviewDate time.Time) error {
	// Check if we have a sheets tab configured for this project
	tabName, exists := PROJECT_SHEETS_TABS[projectName]
	if !exists {
		return fmt.Errorf("no Google Sheets tab configured for project: %s", projectName)
	}

	// Check if credentials file exists
	if _, err := os.Stat(GOOGLE_CREDENTIALS_PATH); os.IsNotExist(err) {
		return fmt.Errorf("Google Sheets credentials not found at %s", GOOGLE_CREDENTIALS_PATH)
	}

	// Read service account credentials
	creds, err := os.ReadFile(GOOGLE_CREDENTIALS_PATH)
	if err != nil {
		return fmt.Errorf("failed to read credentials file: %v", err)
	}

	// Create Google Sheets service with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config, err := google.CredentialsFromJSON(ctx, creds, sheets.SpreadsheetsScope)
	if err != nil {
		// Check for the specific JSON unmarshaling error
		if strings.Contains(err.Error(), "cannot unmarshal string into Go value") {
			fmt.Println("⚠️  Initial credential parsing failed. Retrying with un-escaping...")
			var credsStr string
			if json.Unmarshal(creds, &credsStr) == nil {
				// The file content was a JSON string, so we use the un-escaped version
				config, err = google.CredentialsFromJSON(ctx, []byte(credsStr), sheets.SpreadsheetsScope)
			}
		}
		// If it's still an error after the retry, fail for real
		if err != nil {
			return fmt.Errorf("failed to parse credentials: %v", err)
		}
	}

	srv, err := sheets.NewService(ctx, option.WithCredentials(config))
	if err != nil {
		return fmt.Errorf("failed to create sheets service: %v", err)
	}

	// Prepare row data (matching the format from the monitoring services)
	today := reviewDate.Format("2006/01/02")

	// Find first empty row starting from row 17
	targetRow, err := findFirstEmptyRow(srv, tabName, ctx)
	if err != nil {
		return fmt.Errorf("failed to find empty row in %s: %v", tabName, err)
	}

	// Different row data formats for different tabs
	var rowData []interface{}
	var sheetRange string

	// All tabs now use identical 24-column structure (A-X) with 14-step checkboxes
	switch tabName {
	case "Velo Test", "Mohadin WA_Tool Monitor", "Lawley WA_Tool Monitor":
		// All tabs: 24 columns (A-X) with identical 14-step checkbox structure
		rowData = []interface{}{
			today,        // A: Date
			dropNumber,   // B: Drop Number
			"FALSE", "FALSE", "FALSE", "FALSE", "FALSE", "FALSE", "FALSE", // C-I: Steps 1-7 (checkboxes)
			"FALSE", "FALSE", "FALSE", "FALSE", "FALSE", "FALSE", "FALSE", // J-P: Steps 8-14 (checkboxes)
			0,            // Q: Completed Photos
			14,           // R: Outstanding Photos
			userName,     // S: Contractor Name
			"Processing", // T: Status
			"",           // U: QA Notes
			"",           // V: Comments
			"FALSE",      // W: Resubmitted
			"",           // X: Additional Notes
		}
		sheetRange = fmt.Sprintf("%s!A%d:X%d", tabName, targetRow, targetRow)

	default:
		return fmt.Errorf("unknown tab format for: %s", tabName)
	}

	// Write to specific row instead of appending
	vr := &sheets.ValueRange{
		Values: [][]interface{}{rowData},
	}
	fmt.Printf("📝 Writing to Google Sheets - DR: %s, Tab: %s, Row: %d\n", dropNumber, tabName, targetRow)

	_, err = srv.Spreadsheets.Values.Update(GOOGLE_SHEETS_ID, sheetRange, vr).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	if err != nil {
		return fmt.Errorf("failed to write to Google Sheets (tab: %s, row: %d): %v", tabName, targetRow, err)
	}

	// Apply checkbox data validation to the checkbox columns (C-P and W)
	err = applyCheckboxValidation(srv, tabName, targetRow, ctx)
	if err != nil {
		fmt.Printf("⚠️  Failed to apply checkbox validation (data still written): %v\n", err)
		// Don't return error - data is written, validation is optional
	}

	// Only print success message if everything worked
	fmt.Printf("✅ Added %s to '%s' Google Sheets tab with checkboxes\n", dropNumber, tabName)
	return nil
}

// Copy checkbox formatting from existing rows to ensure proper checkbox display
func applyCheckboxValidation(srv *sheets.Service, tabName string, targetRow int, ctx context.Context) error {
	fmt.Printf("📝 Copying checkbox format to row %d from template row...\n", targetRow)
	
	// Copy checkbox data validation from row 17 (which has working checkboxes)
	// to the new row to ensure proper checkbox display
	
	// Source: Row 17 columns C-P (checkbox columns)
	sourceRange := &sheets.GridRange{
		SheetId:          1654167750, // Velo Test sheet ID
		StartRowIndex:    16, // Row 17 (0-based)
		EndRowIndex:      17, // Row 17 (exclusive end)
		StartColumnIndex: 2,  // Column C (0-based)
		EndColumnIndex:   16, // Column P (exclusive end)
	}
	
	// Destination: New row columns C-P
	destinationRange := &sheets.GridRange{
		SheetId:          1654167750,
		StartRowIndex:    int64(targetRow - 1), // Convert to 0-based
		EndRowIndex:      int64(targetRow),     // Exclusive end
		StartColumnIndex: 2,                    // Column C
		EndColumnIndex:   16,                   // Column P
	}
	
	// Source: Row 17 column W (Resubmitted checkbox)
	sourceRangeW := &sheets.GridRange{
		SheetId:          1654167750,
		StartRowIndex:    16, // Row 17
		EndRowIndex:      17,
		StartColumnIndex: 22, // Column W (0-based)
		EndColumnIndex:   23,
	}
	
	// Destination: New row column W
	destinationRangeW := &sheets.GridRange{
		SheetId:          1654167750,
		StartRowIndex:    int64(targetRow - 1), // Convert to 0-based
		EndRowIndex:      int64(targetRow),
		StartColumnIndex: 22, // Column W
		EndColumnIndex:   23,
	}
	
	// Create batch request to copy formatting for both ranges
	batchRequest := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				CopyPaste: &sheets.CopyPasteRequest{
					Source:      sourceRange,
					Destination: destinationRange,
					PasteType:   "PASTE_DATA_VALIDATION", // Only copy validation rules
				},
			},
			{
				CopyPaste: &sheets.CopyPasteRequest{
					Source:      sourceRangeW,
					Destination: destinationRangeW,
					PasteType:   "PASTE_DATA_VALIDATION",
				},
			},
		},
	}
	
	_, err := srv.Spreadsheets.BatchUpdate(GOOGLE_SHEETS_ID, batchRequest).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to copy checkbox formatting: %v", err)
	}
	
	fmt.Printf("✅ Successfully copied checkbox formatting to row %d\n", targetRow)
	return nil
}

// Handle receipt events for message status updates
func handleReceiptEvent(client *whatsmeow.Client, messageStore *MessageStore, receipt *events.Receipt, logger waLog.Logger) {
	// Receipt events indicate message delivery/read status changes
	// We can use these to detect when messages are processed
	fmt.Printf("📬 Receipt event: Chat=%s, MessageIDs=%v, Timestamp=%v\n", 
		receipt.Chat.String(), receipt.MessageIDs, receipt.Timestamp)
	
	// For resubmissions, we're mainly interested in messages that contain drop numbers
	// The Receipt event alone doesn't give us message content, but we can cross-reference
	// with stored messages to check if any recent messages had 'done' keywords
	
	// Check recent messages from this chat for completion patterns
	checkRecentCompletions(client, messageStore, receipt.Chat.String(), receipt.Timestamp, logger)
}

// Check recent messages for completion patterns and update sheets accordingly
func checkRecentCompletions(client *whatsmeow.Client, messageStore *MessageStore, chatJID string, timestamp time.Time, logger waLog.Logger) {
	// Only process Velo Test group
	veloTestJID := "120363421664266245@g.us"
	if chatJID != veloTestJID {
		return
	}
	
	// Get recent messages from this chat (last 10 messages in past hour)
	since := timestamp.Add(-1 * time.Hour)
	rows, err := messageStore.db.Query(`
		SELECT content, sender, timestamp FROM messages 
		WHERE chat_jid = ? AND timestamp > ? 
		ORDER BY timestamp DESC LIMIT 10
	`, chatJID, since)
	
	if err != nil {
		logger.Warnf("Failed to query recent messages: %v", err)
		return
	}
	defer rows.Close()
	
	// Look for completion patterns in recent messages
	for rows.Next() {
		var content, sender string
		var msgTime time.Time
		
		if err := rows.Scan(&content, &sender, &msgTime); err != nil {
			continue
		}
		
		// Check if this message indicates completion/resubmission
		if isCompletionMessage(content) {
			fmt.Printf("🔔 Found completion message: '%s' from %s\n", content, sender)
			processCompletionMessage(content, chatJID, sender, msgTime, logger)
		}
	}
}

// Check if a message indicates completion or resubmission
func isCompletionMessage(content string) bool {
	content = strings.ToLower(strings.TrimSpace(content))
	
	// Look for completion indicators combined with drop numbers
	hasDropNumber := dropPattern.MatchString(strings.ToUpper(content))
	if !hasDropNumber {
		return false
	}
	
	// Check for completion keywords
	completionWords := []string{"done", "complete", "finished", "ready", "submitted", "resubmitted"}
	for _, word := range completionWords {
		if strings.Contains(content, word) {
			return true
		}
	}
	
	return false
}

// Process completion message and update Google Sheets
func processCompletionMessage(content, chatJID, sender string, timestamp time.Time, logger waLog.Logger) {
	// Extract drop numbers from the completion message
	dropNumbers := dropPattern.FindAllString(strings.ToUpper(content), -1)
	if len(dropNumbers) == 0 {
		return
	}
	
	projectName := getProjectNameByJID(chatJID)
	if projectName == "" {
		return
	}
	
	// For each drop number, update Google Sheets to show resubmission
	for _, dropNumber := range dropNumbers {
		dropNumber = strings.ToUpper(dropNumber)
		fmt.Printf("🔄 Processing completion for %s from %s\n", dropNumber, sender)
		
		// Update Google Sheets to show resubmission status
		err := updateSheetsForResubmission(dropNumber, projectName, logger)
		if err != nil {
			logger.Errorf("❌ Failed to update sheets for %s resubmission: %v", dropNumber, err)
		} else {
			logger.Infof("✅ Updated sheets for %s resubmission", dropNumber)
		}
	}
}

// Update Google Sheets to show resubmission status
func updateSheetsForResubmission(dropNumber, projectName string, logger waLog.Logger) error {
	// Check if we have a sheets tab configured for this project
	tabName, exists := PROJECT_SHEETS_TABS[projectName]
	if !exists {
		return fmt.Errorf("no Google Sheets tab configured for project: %s", projectName)
	}
	
	// Check if credentials file exists
	if _, err := os.Stat(GOOGLE_CREDENTIALS_PATH); os.IsNotExist(err) {
		return fmt.Errorf("Google Sheets credentials not found at %s", GOOGLE_CREDENTIALS_PATH)
	}
	
	// Read service account credentials
	creds, err := os.ReadFile(GOOGLE_CREDENTIALS_PATH)
	if err != nil {
		return fmt.Errorf("failed to read credentials file: %v", err)
	}
	
	// Create Google Sheets service with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	config, err := google.CredentialsFromJSON(ctx, creds, sheets.SpreadsheetsScope)
	if err != nil {
		return fmt.Errorf("failed to parse credentials: %v", err)
	}
	
	srv, err := sheets.NewService(ctx, option.WithCredentials(config))
	if err != nil {
		return fmt.Errorf("failed to create sheets service: %v", err)
	}
	
	// Get all data to find the drop number row
	result, err := srv.Spreadsheets.Values.Get(
		GOOGLE_SHEETS_ID, fmt.Sprintf("%s!A:X", tabName)).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to read sheet data: %v", err)
	}
	
	values := result.Values
	targetRow := -1
	
	// Find the row with this drop number (Column B)
	for rowIndex, row := range values {
		if len(row) > 1 && row[1] != nil {
			if strings.TrimSpace(fmt.Sprintf("%v", row[1])) == dropNumber {
				targetRow = rowIndex + 1 // Convert to 1-based
				break
			}
		}
	}
	
	if targetRow == -1 {
		return fmt.Errorf("drop number %s not found in Google Sheets", dropNumber)
	}
	
	// Update Column W (Resubmitted) to TRUE
	resubmittedRange := fmt.Sprintf("%s!W%d", tabName, targetRow)
	vr := &sheets.ValueRange{
		Values: [][]interface{}{{"TRUE"}},
	}
	
	_, err = srv.Spreadsheets.Values.Update(GOOGLE_SHEETS_ID, resubmittedRange, vr).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()
	
	if err != nil {
		return fmt.Errorf("failed to update resubmission status: %v", err)
	}
	
	fmt.Printf("📊 ✅ Updated Google Sheets: %s Column W=TRUE (Resubmitted)\n", dropNumber)
	return nil
}

// Process drop numbers from message content (enhanced version)
func processDropNumbers(content, chatJID, sender string, timestamp time.Time, logger waLog.Logger) {
	// Check if message is from a tracked project group
	projectName := getProjectNameByJID(chatJID)
	if projectName == "" {
		return // Not from a tracked project group
	}

	// Find all drop numbers in the message
	dropNumbers := dropPattern.FindAllString(content, -1)
	if len(dropNumbers) == 0 {
		return // No drop numbers found
	}

	// Check if this is a completion/resubmission message
	isCompletion := isCompletionMessage(content)
	if isCompletion {
		fmt.Printf("🎯 Completion message detected: '%s' from %s\n", content, sender)
		// Handle completion directly
		processCompletionMessage(content, chatJID, sender, timestamp, logger)
		return
	}

	// Process each drop number (regular new drop processing)
	for _, dropNumber := range dropNumbers {
		dropNumber = strings.ToUpper(dropNumber)

		// Create contractor name from sender
		userName := sender
		if len(sender) > 20 {
			userName = sender[:20]
		}

		// Create QA photo review record in Neon database
		err := createQAPhotoReview(dropNumber, projectName, userName, timestamp)
		if err != nil {
			logger.Errorf("❌ FAILED to create QA review for %s: %v", dropNumber, err)
			continue // Skip to next drop number if database write fails
		}

		// For new drops, write to Google Sheets
		err = writeToGoogleSheets(dropNumber, projectName, userName, timestamp)
		if err != nil {
			logger.Errorf("❌ FAILED to write %s to Google Sheets: %v", dropNumber, err)
			// Continue even if sheets write fails - we still have the database record
		}

		logger.Infof("✅ Processed drop number: %s from %s (project: %s)", dropNumber, sender, projectName)
	}
}

// Handle history sync events
func handleHistorySync(client *whatsmeow.Client, messageStore *MessageStore, historySync *events.HistorySync, logger waLog.Logger) {
	fmt.Printf("Received history sync event with %d conversations\n", len(historySync.Data.Conversations))

	syncedCount := 0
	for _, conversation := range historySync.Data.Conversations {
		// Parse JID from the conversation
		if conversation.ID == nil {
			continue
		}

		chatJID := *conversation.ID

		// Try to parse the JID
		jid, err := types.ParseJID(chatJID)
		if err != nil {
			logger.Warnf("Failed to parse JID %s: %v", chatJID, err)
			continue
		}

		// Get appropriate chat name by passing the history sync conversation directly
		name := GetChatName(client, messageStore, jid, chatJID, conversation, "", logger)

		// Process messages
		messages := conversation.Messages
		if len(messages) > 0 {
			// Update chat with latest message timestamp
			latestMsg := messages[0]
			if latestMsg == nil || latestMsg.Message == nil {
				continue
			}

			// Get timestamp from message info
			timestamp := time.Time{}
			if ts := latestMsg.Message.GetMessageTimestamp(); ts != 0 {
				timestamp = time.Unix(int64(ts), 0)
			} else {
				continue
			}

			messageStore.StoreChat(chatJID, name, timestamp)

			// Store messages
			for _, msg := range messages {
				if msg == nil || msg.Message == nil {
					continue
				}

				// Extract text content
				var content string
				if msg.Message.Message != nil {
					if conv := msg.Message.Message.GetConversation(); conv != "" {
						content = conv
					} else if ext := msg.Message.Message.GetExtendedTextMessage(); ext != nil {
						content = ext.GetText()
					}
				}

				// Extract media info
				var mediaType, filename, url string
				var mediaKey, fileSHA256, fileEncSHA256 []byte
				var fileLength uint64

				if msg.Message.Message != nil {
					mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength = extractMediaInfo(msg.Message.Message)
				}

				// Log the message content for debugging
				logger.Infof("Message content: %v, Media Type: %v", content, mediaType)

				// Skip messages with no content and no media
				if content == "" && mediaType == "" {
					continue
				}

				// Determine sender
				var sender string
				isFromMe := false
				if msg.Message.Key != nil {
					if msg.Message.Key.FromMe != nil {
						isFromMe = *msg.Message.Key.FromMe
					}
					if !isFromMe && msg.Message.Key.Participant != nil && *msg.Message.Key.Participant != "" {
						sender = *msg.Message.Key.Participant
					} else if isFromMe {
						sender = client.Store.ID.User
					} else {
						sender = jid.User
					}
				} else {
					sender = jid.User
				}

				// Store message
				msgID := ""
				if msg.Message.Key != nil && msg.Message.Key.ID != nil {
					msgID = *msg.Message.Key.ID
				}

				// Get message timestamp
				timestamp := time.Time{}
				if ts := msg.Message.GetMessageTimestamp(); ts != 0 {
					timestamp = time.Unix(int64(ts), 0)
				} else {
					continue
				}

				err = messageStore.StoreMessage(
					msgID,
					chatJID,
					sender,
					content,
					timestamp,
					isFromMe,
					mediaType,
					filename,
					url,
					mediaKey,
					fileSHA256,
					fileEncSHA256,
					fileLength,
				)
				if err != nil {
					logger.Warnf("Failed to store history message: %v", err)
				} else {
					syncedCount++
					// Log successful message storage
					if mediaType != "" {
						logger.Infof("Stored message: [%s] %s -> %s: [%s: %s] %s",
							timestamp.Format("2006-01-02 15:04:05"), sender, chatJID, mediaType, filename, content)
					} else {
						logger.Infof("Stored message: [%s] %s -> %s: %s",
							timestamp.Format("2006-01-02 15:04:05"), sender, chatJID, content)
					}
				}
			}
		}
	}

	fmt.Printf("History sync complete. Stored %d messages.\n", syncedCount)
}

// Request history sync from the server
func requestHistorySync(client *whatsmeow.Client) {
	if client == nil {
		fmt.Println("Client is not initialized. Cannot request history sync.")
		return
	}

	if !client.IsConnected() {
		fmt.Println("Client is not connected. Please ensure you are connected to WhatsApp first.")
		return
	}

	if client.Store.ID == nil {
		fmt.Println("Client is not logged in. Please scan the QR code first.")
		return
	}

	// Build and send a history sync request
	historyMsg := client.BuildHistorySyncRequest(nil, 100)
	if historyMsg == nil {
		fmt.Println("Failed to build history sync request.")
		return
	}

	_, err := client.SendMessage(context.Background(), types.JID{
		Server: "s.whatsapp.net",
		User:   "status",
	}, historyMsg)

	if err != nil {
		fmt.Printf("Failed to request history sync: %v\n", err)
	} else {
		fmt.Println("History sync requested. Waiting for server response...")
	}
}

// analyzeOggOpus tries to extract duration and generate a simple waveform from an Ogg Opus file
func analyzeOggOpus(data []byte) (duration uint32, waveform []byte, err error) {
	// Try to detect if this is a valid Ogg file by checking for the "OggS" signature
	// at the beginning of the file
	if len(data) < 4 || string(data[0:4]) != "OggS" {
		return 0, nil, fmt.Errorf("not a valid Ogg file (missing OggS signature)")
	}

	// Parse Ogg pages to find the last page with a valid granule position
	var lastGranule uint64
	var sampleRate uint32 = 48000 // Default Opus sample rate
	var preSkip uint16 = 0
	var foundOpusHead bool

	// Scan through the file looking for Ogg pages
	for i := 0; i < len(data); {
		// Check if we have enough data to read Ogg page header
		if i+27 >= len(data) {
			break
		}

		// Verify Ogg page signature
		if string(data[i:i+4]) != "OggS" {
			// Skip until next potential page
			i++
			continue
		}

		// Extract header fields
		granulePos := binary.LittleEndian.Uint64(data[i+6 : i+14])
		pageSeqNum := binary.LittleEndian.Uint32(data[i+18 : i+22])
		numSegments := int(data[i+26])

		// Extract segment table
		if i+27+numSegments >= len(data) {
			break
		}
		segmentTable := data[i+27 : i+27+numSegments]

		// Calculate page size
		pageSize := 27 + numSegments
		for _, segLen := range segmentTable {
			pageSize += int(segLen)
		}

		// Check if we're looking at an OpusHead packet (should be in first few pages)
		if !foundOpusHead && pageSeqNum <= 1 {
			// Look for "OpusHead" marker in this page
			pageData := data[i : i+pageSize]
			headPos := bytes.Index(pageData, []byte("OpusHead"))
			if headPos >= 0 && headPos+12 < len(pageData) {
				// Found OpusHead, extract sample rate and pre-skip
				// OpusHead format: Magic(8) + Version(1) + Channels(1) + PreSkip(2) + SampleRate(4) + ...
				headPos += 8 // Skip "OpusHead" marker
				// PreSkip is 2 bytes at offset 10
				if headPos+12 <= len(pageData) {
					preSkip = binary.LittleEndian.Uint16(pageData[headPos+10 : headPos+12])
					sampleRate = binary.LittleEndian.Uint32(pageData[headPos+12 : headPos+16])
					foundOpusHead = true
					fmt.Printf("Found OpusHead: sampleRate=%d, preSkip=%d\n", sampleRate, preSkip)
				}
			}
		}

		// Keep track of last valid granule position
		if granulePos != 0 {
			lastGranule = granulePos
		}

		// Move to next page
		i += pageSize
	}

	if !foundOpusHead {
		fmt.Println("Warning: OpusHead not found, using default values")
	}

	// Calculate duration based on granule position
	if lastGranule > 0 {
		// Formula for duration: (lastGranule - preSkip) / sampleRate
		durationSeconds := float64(lastGranule-uint64(preSkip)) / float64(sampleRate)
		duration = uint32(math.Ceil(durationSeconds))
		fmt.Printf("Calculated Opus duration from granule: %f seconds (lastGranule=%d)\n",
			durationSeconds, lastGranule)
	} else {
		// Fallback to rough estimation if granule position not found
		fmt.Println("Warning: No valid granule position found, using estimation")
		durationEstimate := float64(len(data)) / 2000.0 // Very rough approximation
		duration = uint32(durationEstimate)
	}

	// Make sure we have a reasonable duration (at least 1 second, at most 300 seconds)
	if duration < 1 {
		duration = 1
	} else if duration > 300 {
		duration = 300
	}

	// Generate waveform
	waveform = placeholderWaveform(duration)

	fmt.Printf("Ogg Opus analysis: size=%d bytes, calculated duration=%d sec, waveform=%d bytes\n",
		len(data), duration, len(waveform))

	return duration, waveform, nil
}

// min returns the smaller of x or y
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// placeholderWaveform generates a synthetic waveform for WhatsApp voice messages
// that appears natural with some variability based on the duration
func placeholderWaveform(duration uint32) []byte {
	// WhatsApp expects a 64-byte waveform for voice messages
	const waveformLength = 64
	waveform := make([]byte, waveformLength)

	// Seed the random number generator for consistent results with the same duration
	rand.Seed(int64(duration))

	// Create a more natural looking waveform with some patterns and variability
	// rather than completely random values

	// Base amplitude and frequency - longer messages get faster frequency
	baseAmplitude := 35.0
	frequencyFactor := float64(min(int(duration), 120)) / 30.0

	for i := range waveform {
		// Position in the waveform (normalized 0-1)
		pos := float64(i) / float64(waveformLength)

		// Create a wave pattern with some randomness
		// Use multiple sine waves of different frequencies for more natural look
		val := baseAmplitude * math.Sin(pos*math.Pi*frequencyFactor*8)
		val += (baseAmplitude / 2) * math.Sin(pos*math.Pi*frequencyFactor*16)

		// Add some randomness to make it look more natural
		val += (rand.Float64() - 0.5) * 15

		// Add some fade-in and fade-out effects
		fadeInOut := math.Sin(pos * math.Pi)
		val = val * (0.7 + 0.3*fadeInOut)

		// Center around 50 (typical voice baseline)
		val = val + 50

		// Ensure values stay within WhatsApp's expected range (0-100)
		if val < 0 {
			val = 0
		} else if val > 100 {
			val = 100
		}

		waveform[i] = byte(val)
	}

	return waveform
}
