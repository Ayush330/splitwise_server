package logger

import "log"

type Level int

const (
	Debug level = iota
	Info
	Warn
	Error
)

type Logger struct{
	level Level
}

func New(level Level) *Logger{
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	return &Logger{level: Level}
}


func (l *logger) Debug(v ...any){
	if l.level <= Debug{
		log.SetOutput(os.Stdout)
		log.Println(ColorBlue + "[DEBUG] " + fmt.Sprint(v...) + ColorReset)
	}
}

func (l *logger) Info(v ...any){
	if l.level <= Info{
		log.SetOutput(os.Stdout)
		log.Println(ColorGreen + "[INFO] " + fmt.Sprint(v...) + ColorReset)
	}
}

func (l *logger) Warn(v ...any){
	if l.level <= Warn{
		log.SetOutput(os.Stdout)
		log.Println(ColorYellow + "[WARN] " + fmt.Sprint(v...) + ColorReset)
	}
}

func (l *logger) Error(v ...any){
	if l.level <= Error{
		log.SetOutput(os.Stderr)
		log.Println(ColorRed + "[ERROR] " + fmt.Sprint(v...) + ColorReset)
	}
}
