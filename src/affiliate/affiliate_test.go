package affiliate

import (
	"testing"
)

func TestReg1(t *testing.T) {
	var tar target
	tar.parse(`( country in { US,BR,DEBUG} and platform in {iOS } )`)
	str := string(tar.toBytes())
	if str != `{"country":["US","BR"],"platform":["iOS"]}` {
		t.Error(str)
	}
}

func TestReg2(t *testing.T) {
	var tar target
	tar.parse(`( osv in {2.1,2.2} country in { US,BR,DEBUG } and platform in { iOS } )`)
	str := string(tar.toBytes())
	if str != `{"country":["US","BR"],"platform":["iOS"],"osv":["2.1","2.2"]}` {
		t.Error(str)
	}
}
