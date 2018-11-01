package mm010_nrc_api_test

import (
	"fmt"
	"testing"

	api "cc_validator_api"
)

func TestConnection(t *testing.T) {
	_, er := api.NewConnection("COM4", api.Baud9600, true)

	//fmt.Println(r)
	if er != nil {
		fmt.Println(er)
		return
	}
}
