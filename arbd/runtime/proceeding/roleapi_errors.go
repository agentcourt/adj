package proceeding

import "errors"

type participantInputError struct {
	cause error
}

func (err *participantInputError) Error() string {
	return err.cause.Error()
}

func (err *participantInputError) Unwrap() error {
	return err.cause
}

func participantInput(err error) error {
	if err == nil {
		return nil
	}
	var marked *participantInputError
	if errors.As(err, &marked) {
		return err
	}
	return &participantInputError{cause: err}
}

func isParticipantInput(err error) bool {
	var marked *participantInputError
	return errors.As(err, &marked)
}

func roleAPIToolErrorCode(err error) string {
	if isParticipantInput(err) {
		return "tool_failed"
	}
	return "runtime_failure"
}
