package napcatstream

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultChunkSize     int64         = 256 * 1024
	DefaultFileRetention time.Duration = 10 * time.Minute
)

type Upload struct {
	ID            string
	Path          string
	Filename      string
	Size          int64
	SHA256        string
	ChunkSize     int64
	TotalChunks   int
	FileRetention time.Duration
}

type ChunkParams struct {
	StreamID       string `json:"stream_id"`
	ChunkData      string `json:"chunk_data"`
	ChunkIndex     int    `json:"chunk_index"`
	TotalChunks    int    `json:"total_chunks"`
	FileSize       int64  `json:"file_size"`
	ExpectedSHA256 string `json:"expected_sha256"`
	Filename       string `json:"filename"`
	FileRetention  int64  `json:"file_retention"`
}

type CompleteParams struct {
	StreamID      string `json:"stream_id"`
	IsComplete    bool   `json:"is_complete"`
	FileRetention int64  `json:"file_retention"`
}

func NewUpload(path, filename string, chunkSize int64) (*Upload, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("stream upload path is required")
	}
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open stream upload file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect stream upload file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("stream upload source is not a regular file")
	}
	if info.Size() <= 0 {
		return nil, errors.New("stream upload source is empty")
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("hash stream upload file: %w", err)
	}
	streamID, err := randomID()
	if err != nil {
		return nil, err
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = filepath.Base(path)
	}
	size := info.Size()
	return &Upload{
		ID:            streamID,
		Path:          path,
		Filename:      filename,
		Size:          size,
		SHA256:        hex.EncodeToString(hash.Sum(nil)),
		ChunkSize:     chunkSize,
		TotalChunks:   int((size + chunkSize - 1) / chunkSize),
		FileRetention: DefaultFileRetention,
	}, nil
}

func (u *Upload) Chunk(index int) (ChunkParams, error) {
	if u == nil {
		return ChunkParams{}, errors.New("stream upload is nil")
	}
	if index < 0 || index >= u.TotalChunks {
		return ChunkParams{}, fmt.Errorf("stream chunk index %d out of range", index)
	}
	offset := int64(index) * u.ChunkSize
	length := min(u.ChunkSize, u.Size-offset)
	data := make([]byte, int(length))
	file, err := os.Open(u.Path)
	if err != nil {
		return ChunkParams{}, fmt.Errorf("open stream upload chunk: %w", err)
	}
	defer file.Close()
	n, err := file.ReadAt(data, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return ChunkParams{}, fmt.Errorf("read stream upload chunk %d: %w", index, err)
	}
	if n != len(data) {
		return ChunkParams{}, fmt.Errorf("read stream upload chunk %d: got %d bytes, want %d", index, n, len(data))
	}
	return ChunkParams{
		StreamID:       u.ID,
		ChunkData:      base64.StdEncoding.EncodeToString(data),
		ChunkIndex:     index,
		TotalChunks:    u.TotalChunks,
		FileSize:       u.Size,
		ExpectedSHA256: u.SHA256,
		Filename:       u.Filename,
		FileRetention:  u.FileRetention.Milliseconds(),
	}, nil
}

func (u *Upload) Complete() CompleteParams {
	return CompleteParams{
		StreamID:      u.ID,
		IsComplete:    true,
		FileRetention: u.FileRetention.Milliseconds(),
	}
}

func randomID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate stream ID: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}
