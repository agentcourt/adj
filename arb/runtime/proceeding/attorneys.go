package proceeding

import "fmt"

const defaultRemoteSessionCwd = "/home/user"

func attorneyRunInfos(_ Config, _ string) ([]AttorneyRunInfo, error) {
	return []AttorneyRunInfo{
		{Role: "plaintiff", Interface: "lawyerapi"},
		{Role: "defendant", Interface: "lawyerapi"},
	}, nil
}

func validateAttorneyRole(role string) error {
	switch role {
	case "plaintiff", "defendant":
		return nil
	default:
		return fmt.Errorf("unsupported attorney role %q", role)
	}
}
