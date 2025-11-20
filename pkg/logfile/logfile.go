package logfile

import (
	"fmt"
	"os"
	"time"
)

type File struct {
	file *os.File
}

func NewLogFile(filename string) (*File, error) {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &File{file: file}, nil
}

func (l *File) Close() error {
	return l.file.Close()
}

// Write a string to log with date/time prefix
func (l *File) Write(format string, args ...interface{}) error {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, err := fmt.Fprintf(l.file, "%s | %s\n", timestamp, fmt.Sprintf(format, args...))
	return err
}
