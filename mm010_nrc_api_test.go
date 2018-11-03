package mm010_nrc_api_test

import (
	"fmt"
	api "mm010_nrc_api"
	"testing"
)

func TestConnection(t *testing.T) {
	c, er := api.NewConnection("COM4", api.Baud4800, true)

	//fmt.Println(r)
	if er != nil {
		fmt.Println(er)
		return
	}
	//_ = c.Reset()
	//
	//if err != nil {
	//	fmt.Println(err)
	//}

	s, er := c.Status()
	s1,b1,b2, e := c.Dispense(1)

	fmt.Println(s1)
	fmt.Println(b1)
	fmt.Println(b2)
	//fmt.Println(b)
	fmt.Println(e)
	//fmt.Println(r)
	if er != nil {
		fmt.Println(er)
		return
	}

	fmt.Println(s)
}
