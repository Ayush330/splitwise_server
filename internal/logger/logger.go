package logger

import (
	"fmt"
	"log"
	"os"
)

type Level int

const (
	Debug Level = iota
	Info
	Warn
	Error
)

type Logger struct {
	level Level
}

func New(level Level) *Logger {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	return &Logger{level: level}
}

func (l *Logger) Debug(v ...any) {
	if l.level <= Debug {
		log.SetOutput(os.Stdout)
		log.Println(ColorBlue + "[DEBUG] " + fmt.Sprint(v...) + ColorReset)
	}
}

func (l *Logger) Info(v ...any) {
	if l.level <= Info {
		log.SetOutput(os.Stdout)
		log.Println(ColorGreen + "[INFO] " + fmt.Sprint(v...) + ColorReset)
	}
}

func (l *Logger) Warn(v ...any) {
	if l.level <= Warn {
		log.SetOutput(os.Stdout)
		log.Println(ColorYellow + "[WARN] " + fmt.Sprint(v...) + ColorReset)
	}
}

func (l *Logger) Error(v ...any) {
	if l.level <= Error {
		log.SetOutput(os.Stderr)
		log.Println(ColorRed + "[ERROR] " + fmt.Sprint(v...) + ColorReset)
	}
}
