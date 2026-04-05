package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/odvcencio/fyx/compiler/lsp"
)

func runLSP() error {
	server := lsp.NewServer()
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		msg, err := readLSPMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		responses := server.HandleMessage(msg)
		for _, resp := range responses {
			if err := writeLSPMessage(writer, resp); err != nil {
				return err
			}
		}
	}
}

func readLSPMessage(r *bufio.Reader) (json.RawMessage, error) {
	var contentLength int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, _ = strconv.Atoi(val)
		}
	}
	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

func writeLSPMessage(w io.Writer, msg json.RawMessage) error {
	content := string(msg)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))
	_, err := io.WriteString(w, header+content)
	return err
}
