package gateway

import (
	"encoding/base64"
	"encoding/json"
	"unicode/utf8"
)

type transcriptionRequestWire struct {
	Audio           string         `json:"audio"`
	MediaType       string         `json:"mediaType"`
	ProviderOptions map[string]any `json:"providerOptions,omitempty"`
}

func cloneTranscriptionRequest(r TranscriptionRequest) TranscriptionRequest {
	switch v := r.Audio.(type) {
	case TranscriptionBase64:
		r.Audio = v
	case *TranscriptionBase64:
		c := *v
		r.Audio = &c
	case TranscriptionBytes:
		v.Data = append([]byte(nil), v.Data...)
		r.Audio = v
	case *TranscriptionBytes:
		c := *v
		c.Data = append([]byte(nil), v.Data...)
		r.Audio = &c
	}
	r.ProviderOptions = append([]ProviderOption(nil), r.ProviderOptions...)
	return r
}

func transcriptionAudioParts(v TranscriptionAudio) (mediaType, data string, raw []byte, bytesForm bool, err error) {
	switch a := v.(type) {
	case TranscriptionBase64:
		return a.MediaType, a.Data, nil, false, nil
	case *TranscriptionBase64:
		if a == nil {
			break
		}
		return a.MediaType, a.Data, nil, false, nil
	case TranscriptionBytes:
		return a.MediaType, "", a.Data, true, nil
	case *TranscriptionBytes:
		if a == nil {
			break
		}
		return a.MediaType, "", a.Data, true, nil
	}
	return "", "", nil, false, validationError(memberPath("$", "audio"), "must be TranscriptionBase64 or TranscriptionBytes")
}

func preflightTranscriptionRequest(modelID string, r TranscriptionRequest) (int, error) {
	if modelID == "" || !validModelID(modelID) {
		return 0, validationError(memberPath("$", "modelID"), "must be a nonempty provider/model string")
	}
	mediaType, data, raw, bytesForm, err := transcriptionAudioParts(r.Audio)
	if err != nil {
		return 0, err
	}
	if !utf8.ValidString(mediaType) {
		return 0, validationError(memberPath(memberPath("$", "audio"), "mediaType"), "must be valid UTF-8")
	}
	if len(mediaType) > 255 {
		return 0, validationError(memberPath(memberPath("$", "audio"), "mediaType"), "must not exceed 255 bytes")
	}
	dataPath := memberPath(memberPath("$", "audio"), "data")
	encodedLen := 0
	if bytesForm {
		if len(raw) > maxTranscriptionDecodedInputBytes {
			return 0, validationError(dataPath, "must not exceed 8388608 bytes")
		}
		var ok bool
		encodedLen, ok = checkedBase64EncodedLen(len(raw))
		if !ok {
			return 0, transcriptionRequestTooLargeError()
		}
	} else {
		if !utf8.ValidString(data) {
			return 0, validationError(dataPath, "must be valid UTF-8")
		}
		if len(data) > maxStringBytes {
			return 0, validationError(dataPath, "must not exceed 1048576 bytes")
		}
		encodedLen = len(data)
	}
	options, optionErr := encodeProviderOptions(r.ProviderOptions)
	if optionErr != nil {
		return 0, optionErr
	}
	optionsLen := 0
	if len(options) != 0 {
		b, marshalErr := json.Marshal(options)
		if marshalErr != nil {
			return 0, &TransportError{operation: "encode request", cause: marshalErr}
		}
		optionsLen = len(`,"providerOptions":`) + len(b)
	}
	// Raw byte data is standard base64 and needs no JSON escaping; opaque string
	// data uses its exact quoted JSON length.
	dataQuoted := encodedLen + 2
	if !bytesForm {
		dataQuoted = jsonQuotedLength(data)
	}
	size, ok := checkedTranscriptionRequestSize(jsonQuotedLength(mediaType), dataQuoted, optionsLen)
	if !ok {
		return 0, transcriptionRequestTooLargeError()
	}
	return size, nil
}

func checkedTranscriptionRequestSize(mediaTypeQuoted, dataQuoted, optionsSuffix int) (int, bool) {
	// {"audio":<data>,"mediaType":<mediaType><options>}
	total := len(`{"audio":`) + dataQuoted
	var ok bool
	if total, ok = checkedTranscriptionLengthAdd(total, len(`,"mediaType":`)); !ok {
		return 0, false
	}
	if total, ok = checkedTranscriptionLengthAdd(total, mediaTypeQuoted); !ok {
		return 0, false
	}
	if total, ok = checkedTranscriptionLengthAdd(total, optionsSuffix+1); !ok || total > maxRequestBodyBytes {
		return 0, false
	}
	return total, true
}

func checkedTranscriptionLengthAdd(a, b int) (int, bool) {
	if a < 0 || b < 0 || a > maxRequestBodyBytes-b {
		return 0, false
	}
	return a + b, true
}

func transcriptionRequestTooLargeError() error {
	return validationError("$", "encoded request exceeds 16777216 bytes")
}

func prepareTranscriptionRequest(modelID string, r TranscriptionRequest) ([]byte, error) {
	size, err := preflightTranscriptionRequest(modelID, r)
	if err != nil {
		return nil, err
	}
	mediaType, data, raw, bytesForm, _ := transcriptionAudioParts(r.Audio)
	options, optionErr := encodeProviderOptions(r.ProviderOptions)
	if optionErr != nil {
		return nil, optionErr
	}
	payload := make([]byte, 0, size)
	payload = append(payload, `{"audio":`...)
	if bytesForm {
		payload = append(payload, '"')
		start := len(payload)
		payload = payload[:start+base64.StdEncoding.EncodedLen(len(raw))]
		base64.StdEncoding.Encode(payload[start:], raw)
		payload = append(payload, '"')
	} else {
		payload = appendTranscriptionJSONString(payload, data)
	}
	payload = append(payload, `,"mediaType":`...)
	payload = appendTranscriptionJSONString(payload, mediaType)
	if len(options) != 0 {
		encoded, marshalErr := json.Marshal(options)
		if marshalErr != nil {
			return nil, &TransportError{operation: "encode request", cause: marshalErr}
		}
		payload = append(payload, `,"providerOptions":`...)
		payload = append(payload, encoded...)
	}
	payload = append(payload, '}')
	return payload, nil
}

func appendTranscriptionJSONString(dst []byte, value string) []byte {
	dst = append(dst, '"')
	for index := 0; index < len(value); {
		character := value[index]
		if character < utf8.RuneSelf {
			index++
			switch character {
			case '"', '\\':
				dst = append(dst, '\\', character)
			case '\b':
				dst = append(dst, `\b`...)
			case '\f':
				dst = append(dst, `\f`...)
			case '\n':
				dst = append(dst, `\n`...)
			case '\r':
				dst = append(dst, `\r`...)
			case '\t':
				dst = append(dst, `\t`...)
			default:
				if character < 0x20 || character == '<' || character == '>' || character == '&' {
					const hex = "0123456789abcdef"
					dst = append(dst, '\\', 'u', '0', '0', hex[character>>4], hex[character&15])
				} else {
					dst = append(dst, character)
				}
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(value[index:])
		if r == '\u2028' || r == '\u2029' {
			if r == '\u2028' {
				dst = append(dst, `\u2028`...)
			} else {
				dst = append(dst, `\u2029`...)
			}
		} else {
			dst = append(dst, value[index:index+size]...)
		}
		index += size
	}
	return append(dst, '"')
}
