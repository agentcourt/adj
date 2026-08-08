package proceeding

import "testing"

func TestAttorneyRunInfosUseLawyerAPI(t *testing.T) {
	t.Parallel()

	attorneys, err := attorneyRunInfos(Config{}, "")
	if err != nil {
		t.Fatalf("attorneyRunInfos returned error: %v", err)
	}
	if len(attorneys) != 2 {
		t.Fatalf("attorneyRunInfos returned %d roles, want 2", len(attorneys))
	}
	want := map[string]string{"plaintiff": "lawyerapi", "defendant": "lawyerapi"}
	for _, attorney := range attorneys {
		if want[attorney.Role] != attorney.Interface {
			t.Fatalf("attorney role %#v, want interface %q", attorney, want[attorney.Role])
		}
		delete(want, attorney.Role)
	}
	if len(want) != 0 {
		t.Fatalf("attorneyRunInfos missing roles: %#v", want)
	}
}

func TestValidateAttorneyRole(t *testing.T) {
	t.Parallel()

	if err := validateAttorneyRole("plaintiff"); err != nil {
		t.Fatalf("validateAttorneyRole(plaintiff) returned error: %v", err)
	}
	if err := validateAttorneyRole("defendant"); err != nil {
		t.Fatalf("validateAttorneyRole(defendant) returned error: %v", err)
	}
	if err := validateAttorneyRole("observer"); err == nil {
		t.Fatal("validateAttorneyRole(observer) returned nil, want error")
	}
}
