package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

// ErrSinkClosed возвращается приёмником, если Notify вызван после Close.
var ErrSinkClosed = errors.New("audit sink closed")

// FileSink — приёмник событий аудита, дописывающий каждое событие в файл отдельной JSON-строкой.
type FileSink struct {
	mu     sync.Mutex
	file   *os.File
	closed bool
}

// NewFileSink открывает файл для дозаписи. Файл создаётся, если отсутствует.
func NewFileSink(path string) (*FileSink, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("открыть файл аудита %q: %w", path, err)
	}
	return &FileSink{file: f}, nil
}

// Notify сериализует событие в JSON и дописывает его в файл с переводом строки.
func (s *FileSink) Notify(ctx context.Context, ev Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("сериализация события: %w", err)
	}
	data = append(data, '\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrSinkClosed
	}
	if _, err := s.file.Write(data); err != nil {
		return fmt.Errorf("запись события в файл: %w", err)
	}
	return nil
}

// Close закрывает файл.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if err := s.file.Close(); err != nil {
		return fmt.Errorf("закрытие файла аудита: %w", err)
	}
	return nil
}
