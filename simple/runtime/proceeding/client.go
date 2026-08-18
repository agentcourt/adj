package proceeding

import (
	"time"

	"github.com/jsmorph/adj/common/modelrequest"
	openaiapi "github.com/jsmorph/adj/common/openai"
)

type directClientFactory struct{}

func (directClientFactory) New(spec modelrequest.Spec, timeout time.Duration) (ResponseClient, error) {
	client, err := openaiapi.NewForEndpoint(spec.Endpoint, false, timeout)
	if err != nil {
		return nil, err
	}
	if err := client.SetMaxAttempts(1); err != nil {
		return nil, err
	}
	return client, nil
}
