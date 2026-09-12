package openai

import "github.com/agentcourt/adj/common/modelapi"

type Accounting = modelapi.Accounting
type AccountingRecorder = modelapi.AccountingRecorder

func MergeAccounting(values ...Accounting) Accounting {
	return modelapi.MergeAccounting(values...)
}
