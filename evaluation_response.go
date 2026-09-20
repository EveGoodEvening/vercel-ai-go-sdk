package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/big"
	"net/http"
	"sort"
	"strconv"
)

const responseBaseTolerance = 1e-6

type responseNode struct {
	kind byte
	obj  map[string]*responseNode
	arr  []*responseNode
	str  string
	num  json.Number
}

type responseDecodeFailure struct {
	path   string
	reason string
	cause  error
}

func (failure *responseDecodeFailure) Error() string { return failure.reason }
func (failure *responseDecodeFailure) Unwrap() error { return failure.cause }

func decodeResponseNode(body []byte) (*responseNode, *responseDecodeFailure) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	node, failure := readResponseNode(decoder, "$")
	if failure != nil {
		return nil, failure
	}
	if _, err := decoder.Token(); err == nil {
		return nil, &responseDecodeFailure{path: "$", reason: "trailing JSON value"}
	} else if !errors.Is(err, io.EOF) {
		return nil, &responseDecodeFailure{path: "$", reason: "malformed JSON", cause: err}
	}
	return node, nil
}

func readResponseNode(decoder *json.Decoder, path string) (*responseNode, *responseDecodeFailure) {
	token, err := decoder.Token()
	if err != nil {
		return nil, &responseDecodeFailure{path: "$", reason: "malformed JSON", cause: err}
	}
	switch token := token.(type) {
	case nil:
		return &responseNode{kind: 'n'}, nil
	case bool:
		if token {
			return &responseNode{kind: 't'}, nil
		}
		return &responseNode{kind: 'f'}, nil
	case string:
		return &responseNode{kind: 's', str: token}, nil
	case json.Number:
		return &responseNode{kind: '#', num: token}, nil
	case json.Delim:
		switch token {
		case '{':
			object := make(map[string]*responseNode)
			for decoder.More() {
				keyToken, keyErr := decoder.Token()
				if keyErr != nil {
					return nil, &responseDecodeFailure{path: "$", reason: "malformed JSON", cause: keyErr}
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, &responseDecodeFailure{path: "$", reason: "malformed JSON"}
				}
				childPath := memberPath(path, key)
				if _, exists := object[key]; exists {
					return nil, &responseDecodeFailure{path: childPath, reason: "duplicate field"}
				}
				child, childFailure := readResponseNode(decoder, childPath)
				if childFailure != nil {
					return nil, childFailure
				}
				object[key] = child
			}
			if _, closeErr := decoder.Token(); closeErr != nil {
				return nil, &responseDecodeFailure{path: "$", reason: "malformed JSON", cause: closeErr}
			}
			return &responseNode{kind: 'o', obj: object}, nil
		case '[':
			array := make([]*responseNode, 0)
			for decoder.More() {
				child, childFailure := readResponseNode(decoder, indexPath(path, len(array)))
				if childFailure != nil {
					return nil, childFailure
				}
				array = append(array, child)
			}
			if _, closeErr := decoder.Token(); closeErr != nil {
				return nil, &responseDecodeFailure{path: "$", reason: "malformed JSON", cause: closeErr}
			}
			return &responseNode{kind: 'a', arr: array}, nil
		}
	}
	return nil, &responseDecodeFailure{path: "$", reason: "malformed JSON"}
}
func bestEffortResponseIDs(body []byte) (requestID, responseID string) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", ""
	}
	for decoder.More() {
		keyToken, keyErr := decoder.Token()
		if keyErr != nil {
			break
		}
		key, ok := keyToken.(string)
		if !ok {
			break
		}
		value, valueErr := readResponseNode(decoder, memberPath("$", key))
		if valueErr != nil {
			break
		}
		if value.kind != 's' {
			continue
		}
		switch key {
		case "requestId":
			if requestID == "" {
				requestID = value.str
			}
		case "responseId":
			if responseID == "" {
				responseID = value.str
			}
		}
	}
	return requestID, responseID
}

func composeEvaluationResult(modelID string, questions map[string]Question, raw rawEvaluationResponse) (*EvaluationResult, error) {
	body := append([]byte(nil), raw.body...)
	requestID, responseID := bestEffortResponseIDs(body)
	node, failure := decodeResponseNode(body)
	if strictRequestID, strictResponseID := responseIDs(node); node != nil {
		requestID, responseID = strictRequestID, strictResponseID
	}
	invalid := func(path, reason string, cause error) (*EvaluationResult, error) {
		return nil, &ResponseValidationError{cause: cause, statusCode: http.StatusOK, path: path, reason: reason, requestID: requestID, responseID: responseID, bodyTruncated: raw.bodyTruncated, rawResponseBody: body}
	}
	if failure != nil {
		return invalid(failure.path, failure.reason, failure.cause)
	}
	if node.kind != 'o' {
		return invalid("$", "must be an object", nil)
	}
	allowed := map[string]bool{"answers": true, "rounding": true, "usage": true, "warnings": true, "providerMetadata": true}
	if key := firstUnknown(node.obj, allowed); key != "" {
		return invalid(memberPath("$", key), "unknown field", nil)
	}

	answersNode, present := node.obj["answers"]
	if !present {
		return invalid(memberPath("$", "answers"), "required", nil)
	}
	answers, answerFailure := decodeAnswers(answersNode, questions)
	if answerFailure != nil {
		return invalid(answerFailure.path, answerFailure.reason, nil)
	}

	rounding, roundFailure := decodeRounding(node.obj["rounding"])
	if roundFailure != nil {
		return invalid(roundFailure.path, roundFailure.reason, nil)
	}
	usage, usageFailure := decodeUsage(node.obj["usage"])
	if usageFailure != nil {
		return invalid(usageFailure.path, usageFailure.reason, nil)
	}
	warnings, warningFailure := decodeWarnings(node.obj["warnings"])
	if warningFailure != nil {
		return invalid(warningFailure.path, warningFailure.reason, nil)
	}
	metadata, metadataFailure := decodeProviderMetadata(node.obj["providerMetadata"])
	if metadataFailure != nil {
		return invalid(metadataFailure.path, metadataFailure.reason, nil)
	}
	if semanticFailure := validateResponseAnswers(answers, questions, rounding); semanticFailure != nil {
		return invalid(semanticFailure.path, semanticFailure.reason, nil)
	}
	return &EvaluationResult{Answers: answers, Rounding: rounding, Usage: usage, Warnings: warnings, ProviderMetadata: metadata, Response: ResponseMetadata{ModelID: modelID, Headers: nonNilHeaderClone(raw.headers), Body: nonNilByteClone(body)}}, nil
}

func responseIDs(node *responseNode) (string, string) {
	if node == nil || node.kind != 'o' {
		return "", ""
	}
	stringValue := func(key string) string {
		if value := node.obj[key]; value != nil && value.kind == 's' {
			return value.str
		}
		return ""
	}
	return stringValue("requestId"), stringValue("responseId")
}

func nonNilHeaderClone(header http.Header) http.Header {
	clone := header.Clone()
	if clone == nil {
		clone = make(http.Header)
	}
	return clone
}
func nonNilByteClone(body []byte) []byte {
	clone := append([]byte(nil), body...)
	if clone == nil {
		clone = make([]byte, 0)
	}
	return clone
}
func firstUnknown(object map[string]*responseNode, allowed map[string]bool) string {
	keys := sortedNodeKeys(object)
	for _, key := range keys {
		if !allowed[key] {
			return key
		}
	}
	return ""
}
func sortedNodeKeys(object map[string]*responseNode) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type responseFieldFailure struct{ path, reason string }

func decodeAnswers(node *responseNode, questions map[string]Question) (map[string]Answer, *responseFieldFailure) {
	path := memberPath("$", "answers")
	if node.kind == 'n' {
		return nil, &responseFieldFailure{path, "must be non-null"}
	}
	if node.kind != 'o' {
		return nil, &responseFieldFailure{path, "must be an object"}
	}
	for _, id := range sortedNodeKeys(node.obj) {
		if _, ok := questions[id]; !ok {
			return nil, &responseFieldFailure{memberPath(path, id), "answer is unexpected"}
		}
	}
	ids := make([]string, 0, len(questions))
	for id := range questions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	answers := make(map[string]Answer, len(questions))
	for _, id := range ids {
		answerNode, ok := node.obj[id]
		if !ok {
			return nil, &responseFieldFailure{memberPath(path, id), "answer is missing"}
		}
		answer, failure := decodeAnswer(answerNode, memberPath(path, id), questions[id])
		if failure != nil {
			return nil, failure
		}
		answers[id] = answer
	}
	return answers, nil
}

func decodeAnswer(node *responseNode, path string, question Question) (Answer, *responseFieldFailure) {
	if node.kind == 'n' {
		return nil, &responseFieldFailure{path, "must be non-null"}
	}
	if node.kind != 'o' {
		return nil, &responseFieldFailure{path, "must be an object"}
	}
	typeNode, ok := node.obj["type"]
	if !ok {
		return nil, &responseFieldFailure{memberPath(path, "type"), "required"}
	}
	if typeNode.kind == 'n' {
		return nil, &responseFieldFailure{memberPath(path, "type"), "must be non-null"}
	}
	if typeNode.kind != 's' {
		return nil, &responseFieldFailure{memberPath(path, "type"), "must be a string"}
	}
	if typeNode.str != question.questionType() {
		if typeNode.str != "boolean" && typeNode.str != "choice" && typeNode.str != "score" {
			return nil, &responseFieldFailure{memberPath(path, "type"), "answer type is unsupported"}
		}
		return nil, &responseFieldFailure{memberPath(path, "type"), "answer type does not match question"}
	}
	switch typeNode.str {
	case "boolean":
		if key := firstUnknown(node.obj, map[string]bool{"type": true, "probability": true}); key != "" {
			return nil, &responseFieldFailure{memberPath(path, key), "unknown field"}
		}
		probability, failure := requiredFloat(node.obj, "probability", path)
		if failure != nil {
			return nil, failure
		}
		return BooleanAnswer{Probability: probability}, nil
	case "choice":
		if key := firstUnknown(node.obj, map[string]bool{"type": true, "choice": true, "probabilities": true}); key != "" {
			return nil, &responseFieldFailure{memberPath(path, key), "unknown field"}
		}
		choiceNode, present := node.obj["choice"]
		if !present {
			return nil, &responseFieldFailure{memberPath(path, "choice"), "required"}
		}
		if choiceNode.kind == 'n' {
			return nil, &responseFieldFailure{memberPath(path, "choice"), "must be non-null"}
		}
		if choiceNode.kind != 's' {
			return nil, &responseFieldFailure{memberPath(path, "choice"), "must be a string"}
		}
		probabilities, failure := optionalProbabilities(node.obj, path)
		if failure != nil {
			return nil, failure
		}
		return ChoiceAnswer{Choice: choiceNode.str, Probabilities: probabilities}, nil
	case "score":
		if key := firstUnknown(node.obj, map[string]bool{"type": true, "score": true, "probabilities": true}); key != "" {
			return nil, &responseFieldFailure{memberPath(path, key), "unknown field"}
		}
		score, failure := requiredFloat(node.obj, "score", path)
		if failure != nil {
			return nil, failure
		}
		probabilities, probabilityFailure := optionalProbabilities(node.obj, path)
		if probabilityFailure != nil {
			return nil, probabilityFailure
		}
		return ScoreAnswer{Score: score, Probabilities: probabilities}, nil
	}
	return nil, &responseFieldFailure{memberPath(path, "type"), "answer type is unsupported"}
}

func requiredFloat(object map[string]*responseNode, key, path string) (float64, *responseFieldFailure) {
	fieldPath := memberPath(path, key)
	node, ok := object[key]
	if !ok {
		return 0, &responseFieldFailure{fieldPath, "required"}
	}
	if node.kind == 'n' {
		return 0, &responseFieldFailure{fieldPath, "must be non-null"}
	}
	if node.kind != '#' {
		return 0, &responseFieldFailure{fieldPath, "must be a number"}
	}
	value, err := node.num.Float64()
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, &responseFieldFailure{fieldPath, "number must be finite"}
	}
	return value, nil
}

func optionalProbabilities(object map[string]*responseNode, path string) (map[string]float64, *responseFieldFailure) {
	node, present := object["probabilities"]
	if !present {
		return nil, nil
	}
	fieldPath := memberPath(path, "probabilities")
	if node.kind == 'n' {
		return nil, &responseFieldFailure{fieldPath, "must be non-null"}
	}
	if node.kind != 'o' {
		return nil, &responseFieldFailure{fieldPath, "must be an object"}
	}
	if len(node.obj) == 0 {
		return nil, &responseFieldFailure{fieldPath, "must be nonempty"}
	}
	result := make(map[string]float64, len(node.obj))
	for _, key := range sortedNodeKeys(node.obj) {
		valueNode := node.obj[key]
		childPath := memberPath(fieldPath, key)
		if valueNode.kind == 'n' {
			return nil, &responseFieldFailure{childPath, "must be non-null"}
		}
		if valueNode.kind != '#' {
			return nil, &responseFieldFailure{childPath, "must be a number"}
		}
		value, err := valueNode.num.Float64()
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, &responseFieldFailure{childPath, "number must be finite"}
		}
		if value < 0 || value > 1 {
			return nil, &responseFieldFailure{childPath, "must be between 0 and 1"}
		}
		result[key] = value
	}
	return result, nil
}

func decodeRounding(node *responseNode) (*Rounding, *responseFieldFailure) {
	if node == nil {
		return nil, nil
	}
	path := memberPath("$", "rounding")
	if node.kind == 'n' {
		return nil, &responseFieldFailure{path, "must be non-null"}
	}
	if node.kind != 'o' {
		return nil, &responseFieldFailure{path, "must be an object"}
	}
	if key := firstUnknown(node.obj, map[string]bool{"probabilityDecimals": true, "scoreDecimals": true}); key != "" {
		return nil, &responseFieldFailure{memberPath(path, key), "unknown field"}
	}
	probability, failure := optionalDecimal(node.obj, "probabilityDecimals", path)
	if failure != nil {
		return nil, failure
	}
	score, failure := optionalDecimal(node.obj, "scoreDecimals", path)
	if failure != nil {
		return nil, failure
	}
	return &Rounding{ProbabilityDecimals: probability, ScoreDecimals: score}, nil
}

func optionalDecimal(object map[string]*responseNode, key, path string) (*int, *responseFieldFailure) {
	node, present := object[key]
	if !present {
		return nil, nil
	}
	fieldPath := memberPath(path, key)
	if node.kind == 'n' {
		return nil, &responseFieldFailure{fieldPath, "must be non-null"}
	}
	if node.kind != '#' {
		return nil, &responseFieldFailure{fieldPath, "must be an integer"}
	}
	integer, ok := exactJSONInteger(node.num)
	if !ok {
		return nil, &responseFieldFailure{fieldPath, "must be an integer"}
	}
	if integer.Sign() < 0 || !integer.IsInt64() || integer.Int64() > 15 {
		return nil, &responseFieldFailure{fieldPath, "must be between 0 and 15"}
	}
	value := int(integer.Int64())
	return &value, nil
}

func decodeUsage(node *responseNode) (*Usage, *responseFieldFailure) {
	if node == nil {
		return nil, nil
	}
	path := memberPath("$", "usage")
	if node.kind == 'n' {
		return nil, &responseFieldFailure{path, "must be non-null"}
	}
	if node.kind != 'o' {
		return nil, &responseFieldFailure{path, "must be an object"}
	}
	if key := firstUnknown(node.obj, map[string]bool{"inputTokens": true, "outputTokens": true}); key != "" {
		return nil, &responseFieldFailure{memberPath(path, key), "unknown field"}
	}
	input, failure := optionalCount(node.obj, "inputTokens", path)
	if failure != nil {
		return nil, failure
	}
	output, failure := optionalCount(node.obj, "outputTokens", path)
	if failure != nil {
		return nil, failure
	}
	return &Usage{InputTokens: input, OutputTokens: output}, nil
}

func optionalCount(object map[string]*responseNode, key, path string) (*int64, *responseFieldFailure) {
	node, present := object[key]
	if !present {
		return nil, nil
	}
	fieldPath := memberPath(path, key)
	if node.kind == 'n' {
		return nil, &responseFieldFailure{fieldPath, "must be non-null"}
	}
	if node.kind != '#' {
		return nil, &responseFieldFailure{fieldPath, "must be an integer"}
	}
	integer, ok := exactJSONInteger(node.num)
	if !ok || integer.Sign() < 0 || !integer.IsInt64() {
		return nil, &responseFieldFailure{fieldPath, "must be an integer"}
	}
	value := integer.Int64()
	return &value, nil
}

func exactJSONInteger(number json.Number) (*big.Int, bool) {
	text := number.String()
	index := 0
	negative := false
	if index < len(text) && text[index] == '-' {
		negative = true
		index++
	}
	if index == len(text) {
		return nil, false
	}

	digits := make([]byte, 0, 32)
	if text[index] == '0' {
		digits = append(digits, '0')
		index++
		if index < len(text) && text[index] >= '0' && text[index] <= '9' {
			return nil, false
		}
	} else if text[index] >= '1' && text[index] <= '9' {
		for index < len(text) && text[index] >= '0' && text[index] <= '9' {
			digits = append(digits, text[index])
			index++
		}
	} else {
		return nil, false
	}

	fractionDigits := 0
	if index < len(text) && text[index] == '.' {
		index++
		fractionStart := index
		for index < len(text) && text[index] >= '0' && text[index] <= '9' {
			digits = append(digits, text[index])
			fractionDigits++
			index++
		}
		if index == fractionStart {
			return nil, false
		}
	}

	exponentNegative := false
	exponentMagnitude := 0
	if index < len(text) && (text[index] == 'e' || text[index] == 'E') {
		index++
		if index < len(text) && (text[index] == '+' || text[index] == '-') {
			exponentNegative = text[index] == '-'
			index++
		}
		exponentStart := index
		exponentLimit := len(text) + 20
		for index < len(text) && text[index] >= '0' && text[index] <= '9' {
			if exponentMagnitude < exponentLimit {
				exponentMagnitude = exponentMagnitude*10 + int(text[index]-'0')
				if exponentMagnitude > exponentLimit {
					exponentMagnitude = exponentLimit
				}
			}
			index++
		}
		if index == exponentStart {
			return nil, false
		}
	}
	if index != len(text) {
		return nil, false
	}

	firstNonzero := 0
	for firstNonzero < len(digits) && digits[firstNonzero] == '0' {
		firstNonzero++
	}
	if firstNonzero == len(digits) {
		return new(big.Int), true
	}

	effectiveShift := exponentMagnitude - fractionDigits
	if exponentNegative {
		effectiveShift = -exponentMagnitude - fractionDigits
	}
	trailingZeros := 0
	for i := len(digits) - 1; i >= firstNonzero && digits[i] == '0'; i-- {
		trailingZeros++
	}
	if effectiveShift < 0 && -effectiveShift > trailingZeros {
		return nil, false
	}

	resultDigits := len(digits) - firstNonzero + effectiveShift
	if resultDigits <= 0 || resultDigits > 19 {
		return nil, false
	}
	limit := uint64(math.MaxInt64)
	if negative {
		limit++
	}
	var magnitude uint64
	for i := range resultDigits {
		digit := byte(0)
		if source := firstNonzero + i; source < len(digits) {
			digit = digits[source] - '0'
		}
		if magnitude > (limit-uint64(digit))/10 {
			return nil, false
		}
		magnitude = magnitude*10 + uint64(digit)
	}

	integer := new(big.Int).SetUint64(magnitude)
	if negative {
		integer.Neg(integer)
	}
	return integer, true
}

func decodeWarnings(node *responseNode) ([]Warning, *responseFieldFailure) {
	if node == nil {
		return make([]Warning, 0), nil
	}
	path := memberPath("$", "warnings")
	if node.kind == 'n' {
		return nil, &responseFieldFailure{path, "must be non-null"}
	}
	if node.kind != 'a' {
		return nil, &responseFieldFailure{path, "must be an array"}
	}
	warnings := make([]Warning, 0, len(node.arr))
	for index, warningNode := range node.arr {
		warning, failure := decodeWarning(warningNode, indexPath(path, index))
		if failure != nil {
			return nil, failure
		}
		warnings = append(warnings, warning)
	}
	return warnings, nil
}

func decodeWarning(node *responseNode, path string) (Warning, *responseFieldFailure) {
	if node.kind == 'n' {
		return Warning{}, &responseFieldFailure{path, "must be non-null"}
	}
	if node.kind != 'o' {
		return Warning{}, &responseFieldFailure{path, "must be an object"}
	}
	typeNode, present := node.obj["type"]
	if !present {
		return Warning{}, &responseFieldFailure{memberPath(path, "type"), "required"}
	}
	if typeNode.kind == 'n' {
		return Warning{}, &responseFieldFailure{memberPath(path, "type"), "must be non-null"}
	}
	if typeNode.kind != 's' {
		return Warning{}, &responseFieldFailure{memberPath(path, "type"), "must be a string"}
	}
	allowed := map[string]bool{"type": true}
	required := []string{}
	switch typeNode.str {
	case "unsupported", "compatibility":
		allowed["feature"] = true
		allowed["details"] = true
		required = []string{"feature"}
	case "deprecated":
		allowed["setting"] = true
		allowed["message"] = true
		required = []string{"setting", "message"}
	case "other":
		allowed["message"] = true
		required = []string{"message"}
	default:
		return Warning{}, &responseFieldFailure{memberPath(path, "type"), "warning type is unsupported"}
	}
	if key := firstUnknown(node.obj, allowed); key != "" {
		return Warning{}, &responseFieldFailure{memberPath(path, key), "field is forbidden for warning type"}
	}
	for _, key := range required {
		value, ok := node.obj[key]
		if !ok {
			return Warning{}, &responseFieldFailure{memberPath(path, key), "required"}
		}
		if value.kind == 'n' {
			return Warning{}, &responseFieldFailure{memberPath(path, key), "must be non-null"}
		}
		if value.kind != 's' {
			return Warning{}, &responseFieldFailure{memberPath(path, key), "must be a string"}
		}
	}
	warning := Warning{Type: WarningType(typeNode.str)}
	if value := node.obj["feature"]; value != nil {
		if value.kind == 'n' {
			return Warning{}, &responseFieldFailure{memberPath(path, "feature"), "must be non-null"}
		}
		if value.kind != 's' {
			return Warning{}, &responseFieldFailure{memberPath(path, "feature"), "must be a string"}
		}
		warning.Feature = value.str
	}
	if value := node.obj["details"]; value != nil {
		if value.kind == 'n' {
			return Warning{}, &responseFieldFailure{memberPath(path, "details"), "must be non-null"}
		}
		if value.kind != 's' {
			return Warning{}, &responseFieldFailure{memberPath(path, "details"), "must be a string"}
		}
		detail := value.str
		warning.Details = &detail
	}
	if value := node.obj["setting"]; value != nil {
		warning.Setting = value.str
	}
	if value := node.obj["message"]; value != nil {
		warning.Message = value.str
	}
	return warning, nil
}

func decodeProviderMetadata(node *responseNode) (map[string]map[string]any, *responseFieldFailure) {
	if node == nil {
		return nil, nil
	}
	path := memberPath("$", "providerMetadata")
	if node.kind == 'n' {
		return nil, &responseFieldFailure{path, "must be non-null"}
	}
	if node.kind != 'o' {
		return nil, &responseFieldFailure{path, "must be an object"}
	}
	metadata := make(map[string]map[string]any, len(node.obj))
	for _, provider := range sortedNodeKeys(node.obj) {
		value := node.obj[provider]
		providerPath := memberPath(path, provider)
		if value.kind != 'o' {
			return nil, &responseFieldFailure{providerPath, "must be a non-nil object"}
		}
		converted, failure := nodeJSONValue(value, providerPath)
		if failure != nil {
			return nil, failure
		}
		metadata[provider] = converted.(map[string]any)
	}
	return metadata, nil
}

func nodeJSONValue(node *responseNode, path string) (any, *responseFieldFailure) {
	switch node.kind {
	case 'n':
		return nil, nil
	case 't':
		return true, nil
	case 'f':
		return false, nil
	case 's':
		return node.str, nil
	case '#':
		value, err := node.num.Float64()
		if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
			return nil, &responseFieldFailure{path, "number must be finite"}
		}
		return value, nil
	case 'a':
		values := make([]any, len(node.arr))
		for index, child := range node.arr {
			value, failure := nodeJSONValue(child, indexPath(path, index))
			if failure != nil {
				return nil, failure
			}
			values[index] = value
		}
		return values, nil
	case 'o':
		values := make(map[string]any, len(node.obj))
		for _, key := range sortedNodeKeys(node.obj) {
			value, failure := nodeJSONValue(node.obj[key], memberPath(path, key))
			if failure != nil {
				return nil, failure
			}
			values[key] = value
		}
		return values, nil
	}
	return nil, &responseFieldFailure{path, "malformed JSON"}
}

func validateResponseAnswers(answers map[string]Answer, questions map[string]Question, rounding *Rounding) *responseFieldFailure {
	probabilityError, scoreError := 0.0, 0.0
	if rounding != nil {
		if rounding.ProbabilityDecimals != nil {
			probabilityError = 0.5 * math.Pow10(-*rounding.ProbabilityDecimals)
		}
		if rounding.ScoreDecimals != nil {
			scoreError = 0.5 * math.Pow10(-*rounding.ScoreDecimals)
		}
	}
	ids := make([]string, 0, len(questions))
	for id := range questions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		path := memberPath(memberPath("$", "answers"), id)
		switch question := questions[id].(type) {
		case BooleanQuestion:
			if failure := validateBooleanResponse(answers[id].(BooleanAnswer), path); failure != nil {
				return failure
			}
		case *BooleanQuestion:
			if failure := validateBooleanResponse(answers[id].(BooleanAnswer), path); failure != nil {
				return failure
			}
		case ChoiceQuestion:
			if failure := validateChoiceResponse(answers[id].(ChoiceAnswer), question.Criteria, path, probabilityError); failure != nil {
				return failure
			}
		case *ChoiceQuestion:
			if failure := validateChoiceResponse(answers[id].(ChoiceAnswer), question.Criteria, path, probabilityError); failure != nil {
				return failure
			}
		case ScoreQuestion:
			if failure := validateScoreResponse(answers[id].(ScoreAnswer), len(question.Criteria), path, probabilityError, scoreError); failure != nil {
				return failure
			}
		case *ScoreQuestion:
			if failure := validateScoreResponse(answers[id].(ScoreAnswer), len(question.Criteria), path, probabilityError, scoreError); failure != nil {
				return failure
			}
		}
	}
	return nil
}

func validateBooleanResponse(answer BooleanAnswer, path string) *responseFieldFailure {
	if math.IsNaN(answer.Probability) || math.IsInf(answer.Probability, 0) {
		return &responseFieldFailure{memberPath(path, "probability"), "number must be finite"}
	}
	if answer.Probability < 0 || answer.Probability > 1 {
		return &responseFieldFailure{memberPath(path, "probability"), "must be between 0 and 1"}
	}
	return nil
}

func validateChoiceResponse(answer ChoiceAnswer, criteria map[string]any, path string, probabilityError float64) *responseFieldFailure {
	if _, ok := criteria[answer.Choice]; !ok {
		return &responseFieldFailure{memberPath(path, "choice"), "choice is not in criteria"}
	}
	if answer.Probabilities == nil {
		return nil
	}
	probabilityPath := memberPath(path, "probabilities")
	if failure := validateProbabilityKeys(answer.Probabilities, choiceKeys(criteria), probabilityPath); failure != nil {
		return failure
	}
	if failure := validateProbabilitySum(answer.Probabilities, len(criteria), probabilityPath, probabilityError); failure != nil {
		return failure
	}
	selected := answer.Probabilities[answer.Choice]
	for _, key := range choiceKeys(criteria) {
		if answer.Probabilities[key] > selected+responseBaseTolerance {
			return &responseFieldFailure{probabilityPath, "selected choice is not maximal"}
		}
	}
	return nil
}

func choiceKeys(criteria map[string]any) []string {
	keys := make([]string, 0, len(criteria))
	for key := range criteria {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func validateScoreResponse(answer ScoreAnswer, count int, path string, probabilityError, scoreError float64) *responseFieldFailure {
	if math.IsNaN(answer.Score) || math.IsInf(answer.Score, 0) {
		return &responseFieldFailure{memberPath(path, "score"), "number must be finite"}
	}
	if answer.Score < 0 || answer.Score > float64(count-1) {
		return &responseFieldFailure{memberPath(path, "score"), "score is out of range"}
	}
	if answer.Probabilities == nil {
		return nil
	}
	probabilityPath := memberPath(path, "probabilities")
	expected := make([]string, count)
	for index := range expected {
		expected[index] = strconv.Itoa(index)
	}
	if failure := validateProbabilityKeys(answer.Probabilities, expected, probabilityPath); failure != nil {
		return failure
	}
	if failure := validateProbabilitySum(answer.Probabilities, count, probabilityPath, probabilityError); failure != nil {
		return failure
	}
	mean, meanRoundingError := 0.0, 0.0
	for index, key := range expected {
		mean += float64(index) * answer.Probabilities[key]
		meanRoundingError += float64(index) * probabilityError
	}
	if mean < answer.Score-(responseBaseTolerance+meanRoundingError+scoreError) || mean > answer.Score+(responseBaseTolerance+meanRoundingError+scoreError) {
		return &responseFieldFailure{memberPath(path, "score"), "score does not match weighted mean"}
	}
	return nil
}

func validateProbabilityKeys(probabilities map[string]float64, expected []string, path string) *responseFieldFailure {
	expectedSet := make(map[string]bool, len(expected))
	for _, key := range expected {
		expectedSet[key] = true
	}
	for _, key := range expected {
		if _, ok := probabilities[key]; !ok {
			return &responseFieldFailure{memberPath(path, key), "probability key is missing"}
		}
	}
	keys := make([]string, 0, len(probabilities))
	for key := range probabilities {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !expectedSet[key] {
			return &responseFieldFailure{memberPath(path, key), "probability key is unexpected"}
		}
	}
	return nil
}

func validateProbabilitySum(probabilities map[string]float64, count int, path string, probabilityError float64) *responseFieldFailure {
	sum := 0.0
	for _, value := range probabilities {
		sum += value
	}
	tolerance := responseBaseTolerance + float64(count)*probabilityError
	if math.Abs(sum-1) > tolerance {
		return &responseFieldFailure{path, "probabilities must sum to 1 within tolerance"}
	}
	return nil
}
