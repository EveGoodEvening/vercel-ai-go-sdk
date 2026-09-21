package gateway

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

const (
	maxSSELineBytes  = 64 << 10
	maxSSEEventBytes = 1 << 20
)

var (
	errSSELineTooLarge  = errors.New("SSE line exceeds 64 KiB")
	errSSEEventTooLarge = errors.New("SSE event exceeds 1 MiB")
	errSSETruncated     = errors.New("SSE stream ended before an event boundary")
)

type sseEvent struct {
	name string
	id   string
	data []byte
}

type sseParser struct {
	reader      *bufio.Reader
	name        string
	lastEventID string
	data        []byte
	pending     bool
	hasData     bool
}

func newSSEParser(reader io.Reader) *sseParser {
	return &sseParser{reader: bufio.NewReaderSize(reader, maxSSELineBytes+2)}
}

func (p *sseParser) next() (sseEvent, error) {
	for {
		line, eof, err := p.readLine()
		if err != nil {
			return sseEvent{}, err
		}
		if eof {
			if p.pending {
				return sseEvent{}, errSSETruncated
			}
			return sseEvent{}, io.EOF
		}
		if len(line) == 0 {
			if !p.pending {
				continue
			}
			name := p.name
			if name == "" {
				name = "message"
			}
			event := sseEvent{name: name, id: p.lastEventID, data: p.data}
			hasData := p.hasData
			p.name, p.data, p.pending, p.hasData = "", nil, false, false
			if !hasData {
				continue
			}
			return event, nil
		}

		p.pending = true
		if line[0] == ':' {
			continue
		}
		field, value, found := bytes.Cut(line, []byte{':'})
		if !found {
			value = nil
		} else if len(value) != 0 && value[0] == ' ' {
			value = value[1:]
		}
		switch string(field) {
		case "event":
			p.name = string(value)
		case "id":
			if !bytes.ContainsRune(value, 0) {
				p.lastEventID = string(value)
			}
		case "data":
			additional := len(value)
			if p.hasData {
				additional++
			}
			if additional > maxSSEEventBytes-len(p.data) {
				return sseEvent{}, errSSEEventTooLarge
			}
			if p.hasData {
				p.data = append(p.data, '\n')
			}
			p.data = append(p.data, value...)
			p.hasData = true
		}
	}
}

func (p *sseParser) readLine() (line []byte, eof bool, err error) {
	line, err = p.reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return nil, false, errSSELineTooLarge
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	if errors.Is(err, io.EOF) {
		if len(line) == 0 {
			return nil, true, nil
		}
		return nil, false, errSSETruncated
	}
	line = line[:len(line)-1]
	if len(line) != 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	if len(line) > maxSSELineBytes {
		return nil, false, errSSELineTooLarge
	}
	return line, false, nil
}
