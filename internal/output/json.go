package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// JSONRenderer outputs structured JSON messages to stdout.
type JSONRenderer struct{}

// jsonMessage is the structure for JSON output.
type jsonMessage struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

func (r *JSONRenderer) write(level, msg string) {
	data, err := json.Marshal(jsonMessage{Level: level, Message: msg})
	if err != nil {
		// Fallback to raw output if marshalling fails (should not happen)
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

// Success renders a success-level JSON message.
func (r *JSONRenderer) Success(msg string) {
	r.write("success", msg)
}

// Error renders an error-level JSON message.
func (r *JSONRenderer) Error(msg string) {
	r.write("error", msg)
}

// Info renders an info-level JSON message.
func (r *JSONRenderer) Info(msg string) {
	r.write("info", msg)
}

// Warn renders a warn-level JSON message.
func (r *JSONRenderer) Warn(msg string) {
	r.write("warn", msg)
}

// Print renders a plain JSON message with "info" level.
func (r *JSONRenderer) Print(msg string) {
	r.write("info", msg)
}
