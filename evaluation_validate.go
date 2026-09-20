package gateway

import (
	"reflect"
	"sort"
)

func validateEvaluationRequest(modelID string, request EvaluationRequest) *ValidationError {
	if modelID == "" {
		return validationError(memberPath("$", "modelID"), "must be nonempty")
	}
	if err := validateJSONInput(request.State, memberPath("$", "state")); err != nil {
		return err
	}
	questionsPath := memberPath("$", "questions")
	if len(request.Questions) == 0 {
		return validationError(questionsPath, "must be nonempty")
	}
	questionIDs := make([]string, 0, len(request.Questions))
	for id := range request.Questions {
		questionIDs = append(questionIDs, id)
	}
	sort.Strings(questionIDs)
	for _, id := range questionIDs {
		questionPath := memberPath(questionsPath, id)
		if id == "" {
			return validationError(questionPath, "must be nonempty")
		}
		question := request.Questions[id]
		if question == nil || (reflect.ValueOf(question).Kind() == reflect.Pointer && reflect.ValueOf(question).IsNil()) {
			return validationError(questionPath, "must be a non-nil question")
		}
		if err := validateQuestion(question, questionPath); err != nil {
			return err
		}
	}
	if request.ProviderOptions != nil {
		providersPath := memberPath("$", "providerOptions")
		providers := make([]string, 0, len(request.ProviderOptions))
		for provider := range request.ProviderOptions {
			providers = append(providers, provider)
		}
		sort.Strings(providers)
		for _, provider := range providers {
			providerPath := memberPath(providersPath, provider)
			options := request.ProviderOptions[provider]
			if options == nil {
				return validationError(providerPath, "must be a non-nil object")
			}
			if err := validateJSONValue(options, providerPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateQuestion(question Question, path string) *ValidationError {
	switch question := question.(type) {
	case BooleanQuestion:
		return validateBooleanQuestion(question, path)
	case *BooleanQuestion:
		return validateBooleanQuestion(*question, path)
	case ChoiceQuestion:
		return validateChoiceQuestion(question, path)
	case *ChoiceQuestion:
		return validateChoiceQuestion(*question, path)
	case ScoreQuestion:
		return validateScoreQuestion(question, path)
	case *ScoreQuestion:
		return validateScoreQuestion(*question, path)
	default:
		return validationError(path, "question type is unsupported")
	}
}

func validateBooleanQuestion(question BooleanQuestion, path string) *ValidationError {
	if err := validateJSONInput(question.Instructions, memberPath(path, "instructions")); err != nil {
		return err
	}
	if question.Criteria == nil {
		return nil
	}
	criteriaPath := memberPath(path, "criteria")
	if err := validateOptionalJSON(question.Criteria.True, memberPath(criteriaPath, "true")); err != nil {
		return err
	}
	return validateOptionalJSON(question.Criteria.False, memberPath(criteriaPath, "false"))
}

func validateOptionalJSON(value OptionalJSON, path string) *ValidationError {
	if !value.Set {
		if value.Value != nil {
			return validationError(path, "must be omitted when Set is false")
		}
		return nil
	}
	if isJSONNull(value.Value) {
		return nil
	}
	return validateJSONInput(value.Value, path)
}

func validateChoiceQuestion(question ChoiceQuestion, path string) *ValidationError {
	if err := validateJSONInput(question.Instructions, memberPath(path, "instructions")); err != nil {
		return err
	}
	criteriaPath := memberPath(path, "criteria")
	if len(question.Criteria) == 0 {
		return validationError(criteriaPath, "must contain at least one option")
	}
	keys := make([]string, 0, len(question.Criteria))
	for key := range question.Criteria {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := question.Criteria[key]
		if isJSONNull(value) {
			continue
		}
		if err := validateJSONInput(value, memberPath(criteriaPath, key)); err != nil {
			return err
		}
	}
	return nil
}

func validateScoreQuestion(question ScoreQuestion, path string) *ValidationError {
	if err := validateJSONInput(question.Instructions, memberPath(path, "instructions")); err != nil {
		return err
	}
	criteriaPath := memberPath(path, "criteria")
	if len(question.Criteria) < 2 {
		return validationError(criteriaPath, "must contain at least two levels")
	}
	for index, value := range question.Criteria {
		if isJSONNull(value) {
			continue
		}
		if err := validateJSONInput(value, indexPath(criteriaPath, index)); err != nil {
			return err
		}
	}
	return nil
}
