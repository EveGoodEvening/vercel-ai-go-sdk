package gateway

import "encoding/json"

type evaluationRequestWire struct {
	State           any                        `json:"state"`
	Questions       map[string]questionWire    `json:"questions"`
	ProviderOptions *map[string]map[string]any `json:"providerOptions,omitempty"`
}

type questionWire struct {
	Type         string
	Instructions any
	Criteria     any
	HasCriteria  bool
}

func (question questionWire) MarshalJSON() ([]byte, error) {
	if question.HasCriteria {
		return json.Marshal(struct {
			Type         string `json:"type"`
			Instructions any    `json:"instructions"`
			Criteria     any    `json:"criteria"`
		}{question.Type, question.Instructions, question.Criteria})
	}
	return json.Marshal(struct {
		Type         string `json:"type"`
		Instructions any    `json:"instructions"`
	}{question.Type, question.Instructions})
}

func encodeEvaluationRequest(request EvaluationRequest) ([]byte, error) {
	questions := make(map[string]questionWire, len(request.Questions))
	for id, question := range request.Questions {
		questions[id] = encodeQuestion(question)
	}
	wire := evaluationRequestWire{State: normalizeJSONValue(request.State), Questions: questions}
	if request.ProviderOptions != nil {
		providerOptions := make(map[string]map[string]any, len(request.ProviderOptions))
		for provider, options := range request.ProviderOptions {
			providerOptions[provider] = normalizeJSONValue(options).(map[string]any)
		}
		wire.ProviderOptions = &providerOptions
	}
	return json.Marshal(wire)
}

func encodeQuestion(question Question) questionWire {
	switch question := question.(type) {
	case BooleanQuestion:
		return encodeBooleanQuestion(question)
	case *BooleanQuestion:
		return encodeBooleanQuestion(*question)
	case ChoiceQuestion:
		return questionWire{Type: "choice", Instructions: normalizeJSONValue(question.Instructions), Criteria: normalizeJSONValue(question.Criteria), HasCriteria: true}
	case *ChoiceQuestion:
		return questionWire{Type: "choice", Instructions: normalizeJSONValue(question.Instructions), Criteria: normalizeJSONValue(question.Criteria), HasCriteria: true}
	case ScoreQuestion:
		return questionWire{Type: "score", Instructions: normalizeJSONValue(question.Instructions), Criteria: normalizeJSONValue(question.Criteria), HasCriteria: true}
	case *ScoreQuestion:
		return questionWire{Type: "score", Instructions: normalizeJSONValue(question.Instructions), Criteria: normalizeJSONValue(question.Criteria), HasCriteria: true}
	default:
		panic("encodeQuestion called without validation")
	}
}

func encodeBooleanQuestion(question BooleanQuestion) questionWire {
	wire := questionWire{Type: "boolean", Instructions: normalizeJSONValue(question.Instructions)}
	if question.Criteria == nil {
		return wire
	}
	criteria := make(map[string]any, 2)
	if question.Criteria.True.Set {
		criteria["true"] = normalizeJSONValue(question.Criteria.True.Value)
	}
	if question.Criteria.False.Set {
		criteria["false"] = normalizeJSONValue(question.Criteria.False.Value)
	}
	wire.Criteria = criteria
	wire.HasCriteria = true
	return wire
}
