package main

import (
	"fmt"
	"os"
	"time"
)

// fileLogger implémente logger.Logger de Wails en écrivant dans un fichier.
// Un fichier nul (ouverture impossible) rend le journal silencieux plutôt que
// de faire échouer le démarrage : un journal n'est pas une condition de marche.
type fileLogger struct{ file *os.File }

func (l *fileLogger) write(level, message string) {
	if l.file == nil {
		return
	}
	fmt.Fprintf(l.file, "%s [%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), level, message)
}

func (l *fileLogger) Print(message string)   { l.write("PRINT", message) }
func (l *fileLogger) Trace(message string)   { l.write("TRACE", message) }
func (l *fileLogger) Debug(message string)   { l.write("DEBUG", message) }
func (l *fileLogger) Info(message string)    { l.write("INFO", message) }
func (l *fileLogger) Warning(message string) { l.write("WARN", message) }
func (l *fileLogger) Error(message string)   { l.write("ERROR", message) }
func (l *fileLogger) Fatal(message string)   { l.write("FATAL", message) }
