package spec

import (
	"encoding/json"
	"testing"
)

func TestClaimSpecDeclaratoryOnlyDefaultsFalseWhenAbsent(t *testing.T) {
	var claim ClaimSpec
	if err := json.Unmarshal([]byte(`{"claim_id":"claim-1"}`), &claim); err != nil {
		t.Fatal(err)
	}
	if claim.DeclaratoryOnly {
		t.Fatal("absent declaratory_only decoded as true")
	}
}

func TestClaimSpecDeclaratoryOnlyDecodesTrue(t *testing.T) {
	var claim ClaimSpec
	if err := json.Unmarshal([]byte(`{"claim_id":"claim-1","declaratory_only":true}`), &claim); err != nil {
		t.Fatal(err)
	}
	if !claim.DeclaratoryOnly {
		t.Fatal("declaratory_only decoded as false")
	}
}
