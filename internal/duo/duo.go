package duo

import (
	"encoding/json"
	"net/url"

	"github.com/duosecurity/duo_api_golang"

	"github.com/authelia/authelia/internal/logging"
)

// NewDuoAPI create duo API instance
func NewDuoAPI(duoAPI *duoapi.DuoApi) *APIImpl {
	api := new(APIImpl)
	api.DuoApi = duoAPI
	return api
}

// Call call to the DuoAPI
func (d *APIImpl) Call(values url.Values) (*Response, error) {
	_, responseBytes, err := d.DuoApi.SignedCall("POST", "/auth/v2/auth", values)

	if err != nil {
		return nil, err
	}
	logging.Logger().Tracef("Duo Auth Response Data: %s", string(responseBytes))

	var response Response
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}
